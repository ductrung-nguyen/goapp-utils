package filewatcher

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ductrung-nguyen/goapp-utils/pkg/watchapi"
	"github.com/fsnotify/fsnotify"
)

// ReloadWatcher is a lifecycle-safe fsnotify source for configuration reloads.
// It watches files when present and the nearest existing parent while they are absent.
type ReloadWatcher struct {
	watcher *fsnotify.Watcher
	ctx     context.Context
	cancel  context.CancelFunc

	targets map[string]watchapi.Target
	watched map[string]struct{}
	addCh   chan addRequest
	done    chan struct{}
	events  chan watchapi.Event
	errors  chan error
	close   sync.Once
}

type addRequest struct {
	targets []watchapi.Target
	result  chan error
}

var _ watchapi.Source = (*ReloadWatcher)(nil)

// NewReloadWatcher creates and starts a watcher. A nil context is rejected.
func NewReloadWatcher(parent context.Context, targets []watchapi.Target) (*ReloadWatcher, error) {
	if parent == nil {
		return nil, errors.New("filewatcher: nil context")
	}
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(parent)
	r := &ReloadWatcher{
		watcher: w,
		ctx:     ctx,
		cancel:  cancel,
		targets: make(map[string]watchapi.Target),
		watched: make(map[string]struct{}),
		addCh:   make(chan addRequest),
		done:    make(chan struct{}),
		events:  make(chan watchapi.Event, 32),
		errors:  make(chan error, 8),
	}
	if err := r.addTargets(targets); err != nil {
		cancel()
		_ = w.Close()
		return nil, err
	}
	go r.run()
	return r, nil
}

// Factory returns a watchapi.Factory backed by ReloadWatcher.
func Factory(parent context.Context, targets []watchapi.Target) (watchapi.Source, error) {
	return NewReloadWatcher(parent, targets)
}

func (r *ReloadWatcher) Events() <-chan watchapi.Event { return r.events }
func (r *ReloadWatcher) Errors() <-chan error          { return r.errors }

func (r *ReloadWatcher) Add(targets []watchapi.Target) error {
	if targets == nil {
		targets = []watchapi.Target{}
	}
	result := make(chan error, 1)
	select {
	case <-r.done:
		return errors.New("filewatcher: watcher closed")
	case <-r.ctx.Done():
		return r.ctx.Err()
	case r.addCh <- addRequest{targets: append([]watchapi.Target(nil), targets...), result: result}:
	}
	select {
	case err := <-result:
		return err
	case <-r.done:
		return errors.New("filewatcher: watcher closed")
	case <-r.ctx.Done():
		return r.ctx.Err()
	}
}

func (r *ReloadWatcher) Close() error {
	r.close.Do(func() {
		r.cancel()
		<-r.done
	})
	return nil
}

// Wait blocks until the watcher has stopped. It is useful with context-owned sources.
func (r *ReloadWatcher) Wait() { <-r.done }

func (r *ReloadWatcher) addTargets(targets []watchapi.Target) error {
	for _, target := range targets {
		if target.ConfiguredPath == "" {
			continue
		}
		path, err := filepath.Abs(target.ConfiguredPath)
		if err != nil {
			return err
		}
		target.ConfiguredPath = filepath.Clean(path)
		if target.ParentPath != "" {
			parent, err := filepath.Abs(target.ParentPath)
			if err != nil {
				return err
			}
			target.ParentPath = filepath.Clean(parent)
		}
		r.targets[target.ConfiguredPath] = target
		if err := r.arm(target); err != nil {
			return err
		}
	}
	return nil
}

func (r *ReloadWatcher) arm(target watchapi.Target) error {
	path := target.ConfiguredPath
	if _, ok := r.watched[path]; ok {
		return nil
	}
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		if err := r.watcher.Add(path); err != nil {
			return err
		}
		r.watched[path] = struct{}{}
		return nil
	}
	parent := target.ParentPath
	if parent == "" {
		parent = filepath.Dir(path)
	}
	for {
		if info, err := os.Stat(parent); err == nil && info.IsDir() {
			if _, ok := r.watched[parent]; !ok {
				if err := r.watcher.Add(parent); err != nil {
					return err
				}
				r.watched[parent] = struct{}{}
			}
			return nil
		}
		next := filepath.Dir(parent)
		if next == parent {
			return os.ErrNotExist
		}
		parent = next
	}
}

func (r *ReloadWatcher) run() {
	defer close(r.done)
	defer close(r.events)
	defer close(r.errors)
	defer func() {
		if err := r.watcher.Close(); err != nil {
			select {
			case r.errors <- err:
			default:
			}
		}
	}()
	for {
		select {
		case <-r.ctx.Done():
			return
		case req := <-r.addCh:
			err := r.addTargets(req.targets)
			req.result <- err
		case event, ok := <-r.watcher.Events:
			if !ok {
				return
			}
			r.handleEvent(event)
		case err, ok := <-r.watcher.Errors:
			if !ok {
				return
			}
			select {
			case r.errors <- err:
			case <-r.ctx.Done():
				return
			}
		}
	}
}

func (r *ReloadWatcher) handleEvent(event fsnotify.Event) {
	name, err := filepath.Abs(event.Name)
	if err != nil {
		return
	}
	name = filepath.Clean(name)
	eventWasWatched := false
	if _, ok := r.watched[name]; ok {
		eventWasWatched = true
	}
	if event.Op&(fsnotify.Remove|fsnotify.Rename) != 0 {
		delete(r.watched, name)
	}
	for path, target := range r.targets {
		if name == path {
			if event.Op&(fsnotify.Remove|fsnotify.Rename) != 0 {
				delete(r.watched, path)
				_ = r.arm(target)
			}
			if event.Op&fsnotify.Create != 0 {
				_ = r.arm(target)
			}
			r.emit(watchapi.Event{Op: uint32(event.Op), Path: path})
			continue
		}

		// When an absent target's nearest existing parent changes, fsnotify
		// reports the parent path. Preserve the configured target path so
		// consumers can reload the same resource after it is re-armed.
		if !isRelevantAncestorEvent(name, path, r.watched) && (!eventWasWatched || !isAncestor(name, path)) {
			continue
		}
		_ = r.arm(target)
		if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove|fsnotify.Rename) != 0 {
			r.emit(watchapi.Event{Op: uint32(event.Op), Path: path})
		}
	}
}

func isRelevantAncestorEvent(eventPath, targetPath string, watched map[string]struct{}) bool {
	if _, ok := watched[eventPath]; ok {
		return isAncestor(eventPath, targetPath)
	}
	if !isAncestor(eventPath, targetPath) {
		return false
	}

	// Find the nearest watched ancestor strictly above the event. The event's
	// first component below that ancestor identifies the branch that changed.
	// A target is eligible only when it follows the same branch.
	nearest := ""
	for candidate := range watched {
		if !isAncestor(candidate, eventPath) {
			continue
		}
		if len(candidate) > len(nearest) {
			nearest = candidate
		}
	}
	if nearest == "" {
		return true
	}
	eventRel, err := filepath.Rel(nearest, eventPath)
	if err != nil || eventRel == "." {
		return false
	}
	targetRel, err := filepath.Rel(nearest, targetPath)
	if err != nil {
		return false
	}
	eventPart := eventRel
	if idx := strings.IndexRune(eventPart, filepath.Separator); idx >= 0 {
		eventPart = eventPart[:idx]
	}
	targetPart := targetRel
	if idx := strings.IndexRune(targetPart, filepath.Separator); idx >= 0 {
		targetPart = targetPart[:idx]
	}
	return eventPart == targetPart
}

func isAncestor(parent, path string) bool {
	for path != filepath.Dir(path) {
		path = filepath.Dir(path)
		if path == parent {
			return true
		}
	}
	return false
}

func (r *ReloadWatcher) emit(event watchapi.Event) {
	select {
	case r.events <- event:
	case <-r.ctx.Done():
	}
}
