package filewatcher

import (
	"io/fs"
	"io/ioutil"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/ductrung-nguyen/goapp-utils/pkg/logger"
	"github.com/ductrung-nguyen/goapp-utils/pkg/utils"
	"github.com/fsnotify/fsnotify"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/net/context"
)

func TestFileWatcher(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "VCFlag test suite")
}

var _ = Describe("Test filewatcher", func() {

	var watchedFile string
	BeforeEach(func() {
		logger.InitLogger(nil)
		f, err := ioutil.TempFile("/tmp", "test-watcher")
		if err != nil {
			PanicWith(err)
		}
		if err := f.Close(); err != nil {
			PanicWith(err)
		}

		watchedFile = f.Name()
	})

	AfterEach(func() {
		if utils.FileExists(watchedFile) {
			os.Remove(watchedFile)
		}
	})

	When("file exists already, and then removed, and added again", func() {
		It("should detect all changes", func() {
			events := []fsnotify.Event{}

			err := ioutil.WriteFile(watchedFile, []byte("This is a dummy file"), fs.ModePerm)
			Expect(err).NotTo(HaveOccurred())

			fw, err := New(watchedFile, func(f *FileWatcher, event fsnotify.Event) {
				events = append(events, event)
			}, nil)

			Expect(err).ShouldNot(HaveOccurred())
			Expect(fw).ToNot(BeNil())

			fw.CheckFileInterval = 5 * time.Millisecond

			ctx, cancel := context.WithCancel(context.TODO())

			// start the watcher
			go fw.Watch(ctx)

			// start doing the change on the file in another go-routine
			go func() {
				// wait a bit for the watcher to detect the file
				time.Sleep(10 * time.Millisecond)

				os.Remove(watchedFile)
				time.Sleep(10 * time.Millisecond)

				// note that this action only triggers the event CREATE
				// the event WRITE is paused until another event raised
				if err := ioutil.WriteFile(watchedFile, []byte("Trigger the action CREATE + WRITE"), fs.ModePerm); err != nil {
					PanicWith(err)
				}
				time.Sleep(10 * time.Millisecond)

				if err := ioutil.WriteFile(watchedFile, []byte("Execute action WRITE"), fs.ModePerm); err != nil {
					PanicWith(err)
				}
				time.Sleep(10 * time.Millisecond)

				// remove the file again, then create again
				os.Remove(watchedFile)
				time.Sleep(10 * time.Millisecond)

				// note that this action only triggers the event CREATE
				// the event WRITE is paused until another event raised
				if err := ioutil.WriteFile(watchedFile, []byte("Removed and created again"), fs.ModePerm); err != nil {
					PanicWith(err)
				}
				time.Sleep(10 * time.Millisecond)
			}()

			time.Sleep(400 * time.Millisecond)

			cancel()

			operationWhenCreatingFile := fsnotify.Write
			if runtime.GOOS == "darwin" {
				operationWhenCreatingFile = fsnotify.Chmod
			}

			Expect(events).To(Equal([]fsnotify.Event{
				{Name: watchedFile, Op: fsnotify.Remove},
				{Name: watchedFile, Op: fsnotify.Create},
				{Name: watchedFile, Op: operationWhenCreatingFile},
				{Name: watchedFile, Op: fsnotify.Write},

				{Name: watchedFile, Op: fsnotify.Remove},
				{Name: watchedFile, Op: fsnotify.Create},
				// {Name: watchedFile, Op: fsnotify.Write},
			}))

		})
	})
})
