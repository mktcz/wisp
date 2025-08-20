package runner

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/mktcz/wisp/internal/config"
	"github.com/mktcz/wisp/internal/process"
	"github.com/mktcz/wisp/internal/watcher"
)

type Runner struct {
	config    *config.Config
	managers  map[string]*process.Manager
	watchers  map[string]*watcher.Watcher
	mu        sync.RWMutex
	done      chan struct{}
	interrupt chan os.Signal
}

func New(cfg *config.Config) *Runner {
	return &Runner{
		config:    cfg,
		managers:  make(map[string]*process.Manager),
		watchers:  make(map[string]*watcher.Watcher),
		done:      make(chan struct{}),
		interrupt: make(chan os.Signal, 1),
	}
}

func (r *Runner) Run(appNames ...string) error {
	appsToRun := make(map[string]*config.App)

	if len(appNames) > 0 {

		for _, name := range appNames {
			app, exists := r.config.Apps[name]
			if !exists {
				return fmt.Errorf("app '%s' not found in configuration", name)
			}
			appsToRun[name] = app
		}
	} else {

		appsToRun = r.config.Apps
	}

	if len(appsToRun) == 0 {
		return fmt.Errorf("no applications configured")
	}

	signal.Notify(r.interrupt, os.Interrupt, syscall.SIGTERM)

	var wg sync.WaitGroup
	startErrors := make(chan error, len(appsToRun))

	for name, app := range appsToRun {
		wg.Add(1)
		go func(appName string, appConfig *config.App) {
			defer wg.Done()
			if err := r.startApp(appName, appConfig); err != nil {
				startErrors <- fmt.Errorf("[%s] %w", appName, err)
			}
		}(name, app)
	}

	wg.Wait()
	close(startErrors)

	var startupFailed bool
	for err := range startErrors {
		log.Printf("Error: %v", err)
		startupFailed = true
	}

	if startupFailed {
		r.Shutdown()
		return fmt.Errorf("one or more applications failed to start")
	}

	log.Printf("Wisp is running %d application(s). Press Ctrl+C to stop.", len(appsToRun))

	select {
	case <-r.interrupt:
		log.Println("\nReceived interrupt signal, shutting down...")
		r.Shutdown()
	case <-r.done:
		log.Println("Runner stopped")
	}

	return nil
}

func (r *Runner) startApp(name string, app *config.App) error {
	log.Printf("[%s] Starting application...", name)

	manager := process.NewManager(app)

	manager.SetDelays(1*time.Second, 500*time.Millisecond)

	r.mu.Lock()
	r.managers[name] = manager
	r.mu.Unlock()

	if err := manager.Restart(); err != nil {
		return fmt.Errorf("failed to start: %w", err)
	}

	fileWatcher, err := watcher.New(300 * time.Millisecond)
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}

	if err := fileWatcher.SetExcludes(app.ExcludeDir, app.ExcludeFile, app.ExcludeRegex); err != nil {
		return fmt.Errorf("failed to set excludes: %w", err)
	}
	fileWatcher.SetFollowSymlink(app.FollowSymlink)

	r.mu.Lock()
	r.watchers[name] = fileWatcher
	r.mu.Unlock()

	if err := fileWatcher.Watch(app.WatchDir); err != nil {
		return fmt.Errorf("failed to watch directory %s: %w", app.WatchDir, err)
	}

	fileWatcher.Start()
	log.Printf("[%s] Watching directory: %s", name, app.WatchDir)

	go r.handleFileChanges(name, manager, fileWatcher)

	return nil
}

func (r *Runner) handleFileChanges(appName string, manager *process.Manager, fileWatcher *watcher.Watcher) {
	for {
		select {
		case <-fileWatcher.Events:
			log.Printf("[%s] File change detected, rebuilding...", appName)

			if err := manager.Restart(); err != nil {
				log.Printf("[%s] Restart failed: %v", appName, err)

			}

		case err := <-fileWatcher.Errors:
			log.Printf("[%s] Watcher error: %v", appName, err)

		case <-r.done:
			return
		}
	}
}

func (r *Runner) RunSingle(appName string) error {
	return r.Run(appName)
}

func (r *Runner) Shutdown() {
	log.Println("Shutting down all applications...")

	close(r.done)

	r.mu.RLock()
	watchers := make([]*watcher.Watcher, 0, len(r.watchers))
	for _, w := range r.watchers {
		watchers = append(watchers, w)
	}
	r.mu.RUnlock()

	for _, w := range watchers {
		if err := w.Stop(); err != nil {
			log.Printf("Error stopping watcher: %v", err)
		}
	}

	r.mu.RLock()
	managers := make([]*process.Manager, 0, len(r.managers))
	names := make([]string, 0, len(r.managers))
	for name, m := range r.managers {
		managers = append(managers, m)
		names = append(names, name)
	}
	r.mu.RUnlock()

	var wg sync.WaitGroup
	for i, m := range managers {
		wg.Add(1)
		go func(manager *process.Manager, name string) {
			defer wg.Done()
			if err := manager.Stop(); err != nil {
				log.Printf("[%s] Error stopping process: %v", name, err)
			}
		}(m, names[i])
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("All applications stopped successfully")
	case <-time.After(10 * time.Second):
		log.Println("Warning: Shutdown timeout exceeded")
	}

	for _, m := range managers {
		m.CleanUp()
	}
}

func (r *Runner) GetRunningApps() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	apps := make([]string, 0, len(r.managers))
	for name, manager := range r.managers {
		if manager.IsRunning() {
			apps = append(apps, name)
		}
	}
	return apps
}
