package main

import (
	"context"
	"fmt"

	"github.com/ductrung-nguyen/goapp-utils/pkg/filewatcher"
	"github.com/fsnotify/fsnotify"
)

func main() {
	var handler = func(f *filewatcher.FileWatcher, event fsnotify.Event) {
		fmt.Printf("Receive event: %#v\n", event)
	}

	// watch file "my_file.txt" even if it does not exist yet
	fw, err := filewatcher.New("my_file.txt", handler, nil)
	if err == nil {
		fw.Watch(context.Background())
	}
}
