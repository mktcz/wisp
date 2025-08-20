package watcher

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

type Watcher struct {
	watcher       *fsnotify.Watcher
	debounceTime  time.Duration
	ignoreDirs    []string
	excludeDirs   []string
	excludeFiles  []string
	excludeRegex  []*regexp.Regexp
	followSymlink bool
	Events        chan struct{}
	Errors        chan error
	done          chan struct{}
}

func New(debounceTime time.Duration) (*Watcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create watcher: %w", err)
	}

	w := &Watcher{
		watcher:      fsWatcher,
		debounceTime: debounceTime,
		ignoreDirs: []string{
			"tmp", ".tmp", "vendor", ".git", ".idea", ".vscode",
			"node_modules", "dist", "build", ".next", ".nuxt",
		},
		Events: make(chan struct{}, 1),
		Errors: make(chan error, 10),
		done:   make(chan struct{}),
	}

	return w, nil
}

func (w *Watcher) SetExcludes(dirs []string, files []string, regexPatterns []string) error {
	w.excludeDirs = append(w.excludeDirs, dirs...)
	w.excludeFiles = files

	for _, pattern := range regexPatterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("invalid regex pattern %s: %w", pattern, err)
		}
		w.excludeRegex = append(w.excludeRegex, re)
	}

	return nil
}

func (w *Watcher) SetFollowSymlink(follow bool) {
	w.followSymlink = follow
}

func (w *Watcher) Watch(dir string) error {

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() && w.shouldIgnore(path) {
			return filepath.SkipDir
		}

		if info.IsDir() {
			if err := w.watcher.Add(path); err != nil {
				log.Printf("Warning: failed to watch %s: %v", path, err)
			}
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk directory %s: %w", dir, err)
	}

	return nil
}

func (w *Watcher) Start() {
	go w.run()
}

func (w *Watcher) Stop() error {
	close(w.done)
	return w.watcher.Close()
}

func (w *Watcher) run() {
	var (
		timer       *time.Timer
		timerActive bool
	)

	for {
		select {
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}

			if w.shouldSkipEvent(event) {
				continue
			}

			if timerActive && timer != nil {
				timer.Stop()
			}

			timer = time.AfterFunc(w.debounceTime, func() {
				select {
				case w.Events <- struct{}{}:
				default:

				}
				timerActive = false
			})
			timerActive = true

			if event.Op&fsnotify.Create == fsnotify.Create {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					if !w.shouldIgnore(event.Name) {
						w.watcher.Add(event.Name)
					}
				}
			}

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			select {
			case w.Errors <- err:
			default:
				log.Printf("Error channel full, dropping error: %v", err)
			}

		case <-w.done:
			if timer != nil {
				timer.Stop()
			}
			return
		}
	}
}

func (w *Watcher) shouldIgnore(path string) bool {
	base := filepath.Base(path)

	if strings.HasPrefix(base, ".") && base != "." && base != ".." {
		return true
	}

	for _, ignored := range w.ignoreDirs {
		if base == ignored {
			return true
		}
	}

	for _, excluded := range w.excludeDirs {
		if base == excluded || strings.Contains(path, "/"+excluded+"/") {
			return true
		}
	}

	for _, re := range w.excludeRegex {
		if re.MatchString(path) {
			return true
		}
	}

	return false
}

func (w *Watcher) shouldSkipEvent(event fsnotify.Event) bool {

	if event.Op == fsnotify.Chmod {
		return true
	}

	base := filepath.Base(event.Name)
	ext := filepath.Ext(event.Name)

	if strings.HasSuffix(base, "~") || strings.HasPrefix(base, ".#") {
		return true
	}

	if ext == ".swp" || ext == ".swo" || ext == ".swx" {
		return true
	}

	if strings.Contains(event.Name, "test.test") {
		return true
	}

	for _, excluded := range w.excludeFiles {
		if matched, _ := filepath.Match(excluded, base); matched {
			return true
		}
	}

	for _, re := range w.excludeRegex {
		if re.MatchString(event.Name) {
			return true
		}
	}

	return false
}
