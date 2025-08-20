package runner

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/mktcz/wisp/internal/config"
	"github.com/mktcz/wisp/internal/process"
	"github.com/mktcz/wisp/internal/session"
	"github.com/mktcz/wisp/internal/watcher"
)

type Runner struct {
	config     *config.Config
	managers   map[string]*process.Manager
	watchers   map[string]*watcher.Watcher
	sessionDir string
	mu         sync.RWMutex
	done       chan struct{}
	interrupt  chan os.Signal
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
	// Generate session directory for this run
	sessionDir, err := session.GenerateSessionDir()
	if err != nil {
		return fmt.Errorf("failed to create session directory: %w", err)
	}
	r.sessionDir = sessionDir

	appsToRun := make(map[string]*config.App)

	if len(appNames) > 0 {

		for _, name := range appNames {
			app, exists := r.config.Apps[name]
			if !exists {
				return fmt.Errorf("app '%s' not found in configuration", name)
			}
			// Create a copy of the app config with translated paths
			appCopy := *app
			r.translatePaths(&appCopy)
			appsToRun[name] = &appCopy
		}
	} else {
		// Copy all apps with translated paths
		for name, app := range r.config.Apps {
			appCopy := *app
			r.translatePaths(&appCopy)
			appsToRun[name] = &appCopy
		}
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

	// Clean up process artifacts
	for _, m := range managers {
		m.CleanUp()
	}

	// Clean up session directories
	r.cleanupSessionDirectories()
}

// translatePaths converts relative ./tmp paths to session directory paths
func (r *Runner) translatePaths(app *config.App) {
	if r.sessionDir == "" {
		return
	}
	
	// Translate build command paths
	if strings.Contains(app.Cmd, "./tmp/") {
		app.Cmd = strings.ReplaceAll(app.Cmd, "./tmp/", r.sessionDir+"/")
	}
	if strings.Contains(app.BuildCmd, "./tmp/") {
		app.BuildCmd = strings.ReplaceAll(app.BuildCmd, "./tmp/", r.sessionDir+"/")
	}
	
	// Translate binary path
	if strings.HasPrefix(app.Bin, "./tmp/") {
		app.Bin = strings.Replace(app.Bin, "./tmp/", r.sessionDir+"/", 1)
	}
	
	// Translate tmp_dir
	if app.TmpDir == "./tmp" {
		app.TmpDir = r.sessionDir
	}
}

func (r *Runner) cleanupSessionDirectories() {
	// Clean up the session directory if it exists
	if r.sessionDir != "" {
		if err := session.CleanupSessionDir(r.sessionDir); err != nil {
			log.Printf("Warning: Failed to clean up session directory %s: %v", r.sessionDir, err)
		} else {
			log.Printf("Cleaned up session directory: %s", r.sessionDir)
		}
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
