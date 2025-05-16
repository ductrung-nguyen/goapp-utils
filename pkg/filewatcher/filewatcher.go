package filewatcher

import (
	"context"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"golang.org/x/sys/unix"
	"rndwww.nce.amadeus.net/git/SPLUNK/goapp-utils/pkg/utils"
)

// FileWatcher is a helper struct that helps us watching a file
// event if the file does't exist yet
type FileWatcher struct {
	sync.Mutex
	filePath       string
	fileExist      bool
	inWatchedQueue bool

	watcher *fsnotify.Watcher

	onChangeHandler func(f *FileWatcher, event fsnotify.Event)
	onErrorHandler  func(f *FileWatcher, err error) bool

	// the interval that we will check the existence of the file again
	// default = 5s, can be adjustable, especially for unittests
	CheckFileInterval time.Duration
}

// Create new watcher for a file
// with some pre-defined handlers
func New(
	filePath string,
	onChangeHandler func(f *FileWatcher, event fsnotify.Event),
	onErrorHandler func(f *FileWatcher, err error) bool,
) (*FileWatcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	f := FileWatcher{
		filePath:  filePath,
		fileExist: false, inWatchedQueue: false,
		watcher:           w,
		onChangeHandler:   onChangeHandler,
		onErrorHandler:    onErrorHandler,
		CheckFileInterval: 5 * time.Second,
	}

	return &f, nil
}

// the watched file exists ot not?
func (f *FileWatcher) FileExist() bool {
	f.Lock()
	defer f.Unlock()
	return f.fileExist
}

// set the existence of the watched file
func (f *FileWatcher) SetFileExist(value bool) {
	f.Lock()
	defer f.Unlock()
	f.fileExist = value
}

// is the file in the watched queue?
func (f *FileWatcher) InWatchedQueue() bool {
	f.Lock()
	defer f.Unlock()
	return f.inWatchedQueue
}

// update the status of the file in the watched queue
func (f *FileWatcher) SetInWatchedQueue(value bool) {
	f.Lock()
	defer f.Unlock()
	f.inWatchedQueue = value
}

func (f *FileWatcher) getOnChangeHandler() func(f *FileWatcher, event fsnotify.Event) {
	f.Lock()
	defer f.Unlock()
	return f.onChangeHandler
}

// Start watching the file
// Even when the file is deleted and re-created, the watcher can still put an eye on it
// until the context is done
func (f *FileWatcher) Watch(ctx context.Context) {
	if !f.FileExist() && utils.FileExists(f.filePath) {
		f.SetFileExist(true)
	}

	done := make(chan bool)
	go func() {
		defer close(done)
		for {
			select {
			case event, ok := <-f.watcher.Events:
				if !ok {
					return
				}
				switch event.Op {
				case fsnotify.Create:
					f.SetFileExist(true)
				case fsnotify.Remove:
					f.SetFileExist(false)
					f.SetInWatchedQueue(false)
				}
				if f.getOnChangeHandler() != nil {
					f.getOnChangeHandler()(f, event)
				}
			case err, ok := <-f.watcher.Errors:
				if !ok {
					return
				}
				if f.onErrorHandler != nil {
					f.onErrorHandler(f, err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		for {
			if !f.FileExist() && utils.FileExists(f.filePath) {
				f.SetFileExist(true)
				if f.getOnChangeHandler() != nil {
					f.getOnChangeHandler()(f, fsnotify.Event{Name: f.filePath, Op: fsnotify.Create})
				}
			}
			if f.FileExist() && !f.InWatchedQueue() {
				if err := f.watcher.Add(f.filePath); err != nil {
					if err == unix.ENOENT {
						f.SetFileExist(false)
					}
				} else {
					f.SetInWatchedQueue(true)
				}
			}

			time.Sleep(f.CheckFileInterval)
		}
	}()

	<-done
}
