package filewatcher

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/ductrung-nguyen/goapp-utils/pkg/logger"
	"github.com/ductrung-nguyen/goapp-utils/pkg/utils"
	"github.com/ductrung-nguyen/goapp-utils/pkg/watchapi"
	"github.com/fsnotify/fsnotify"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/net/context"
)

func TestFileWatcher(t *testing.T) {
	RegisterFailHandler(Fail)
	suiteConfig, repoterConfig := GinkgoConfiguration()
	suiteConfig.PollProgressAfter = 1 * time.Second
	repoterConfig.FullTrace = true
	RunSpecs(t, "File watcher test suite")
}

var _ = Describe("Test filewatcher", func() {
	var watchedFile string
	BeforeEach(func() {
		logger.InitLogger(nil)
		f, err := os.CreateTemp("/tmp", "test-watcher")
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
			_ = os.Remove(watchedFile)
		}
	})

	When("file exists already, and then removed, and added again", func() {
		It("should detect all changes", func() {
			events := []fsnotify.Event{}
			locker := sync.Mutex{}

			err := os.WriteFile(watchedFile, []byte("This is a dummy file"), fs.ModePerm)
			Expect(err).NotTo(HaveOccurred())

			fw, err := New(watchedFile, func(f *FileWatcher, event fsnotify.Event) {
				locker.Lock()
				defer locker.Unlock()
				events = append(events, event)
			}, nil)

			Expect(err).ShouldNot(HaveOccurred())
			Expect(fw).ToNot(BeNil())

			fw.CheckFileInterval = 5 * time.Millisecond

			ctx, cancel := context.WithCancel(context.TODO())
			defer cancel()

			// start the watcher
			go fw.Watch(ctx)

			// Drive each filesystem transition only after the watcher has reached
			// the state required to observe it. Fixed sleeps made this test lose
			// the create event when the polling goroutine had not re-armed the
			// path yet.
			eventCount := func(op fsnotify.Op) int {
				locker.Lock()
				defer locker.Unlock()
				count := 0
				for _, event := range events {
					if event.Name == watchedFile && event.Op == op {
						count++
					}
				}
				return count
			}
			waitForEvent := func(op fsnotify.Op, occurrence int) {
				Eventually(func() bool { return eventCount(op) >= occurrence }).Should(BeTrue())
			}

			Eventually(fw.InWatchedQueue).Should(BeTrue())
			Expect(os.Remove(watchedFile)).To(Succeed())
			Eventually(func() bool {
				return eventCount(fsnotify.Remove) >= 1 && !fw.InWatchedQueue()
			}).Should(BeTrue())

			Expect(os.WriteFile(watchedFile, []byte("Trigger the action CREATE + WRITE"), fs.ModePerm)).To(Succeed())
			waitForEvent(fsnotify.Create, 1)
			Eventually(fw.InWatchedQueue).Should(BeTrue())
			Expect(os.WriteFile(watchedFile, []byte("Execute action WRITE"), fs.ModePerm)).To(Succeed())
			waitForEvent(fsnotify.Write, 1)

			Expect(os.Remove(watchedFile)).To(Succeed())
			Eventually(func() bool {
				return eventCount(fsnotify.Remove) >= 2 && !fw.InWatchedQueue()
			}).Should(BeTrue())
			Expect(os.WriteFile(watchedFile, []byte("Removed and created again"), fs.ModePerm)).To(Succeed())
			waitForEvent(fsnotify.Create, 2)

			func() {
				locker.Lock()
				defer locker.Unlock()
				expectedEventsInOrder := []fsnotify.Event{
					{Name: watchedFile, Op: fsnotify.Remove},
					{Name: watchedFile, Op: fsnotify.Create},
					{Name: watchedFile, Op: fsnotify.Write},
					{Name: watchedFile, Op: fsnotify.Remove},
					{Name: watchedFile, Op: fsnotify.Create},
				}
				lastPos := -1
				for idx, expectedEvent := range expectedEventsInOrder {
					found := false
					for i := lastPos + 1; i < len(events); i++ {
						if expectedEvent.Name == events[i].Name && expectedEvent.Op == events[i].Op {
							lastPos = i
							found = true
							break
						}
					}
					By(fmt.Sprintf("Checking the existence of event #%d %v", idx, expectedEvent.Op))
					Expect(found).To(BeTrue())
				}
			}()
		})
	})
})

func TestReloadWatcherMapsMissingParentCreateToTarget(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "one", "two", "config")

	rw, err := NewReloadWatcher(context.Background(), []watchapi.Target{{
		ConfiguredPath: target,
		ParentPath:     filepath.Dir(target),
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := rw.Close(); err != nil {
			t.Errorf("close reload watcher: %v", err)
		}
	}()

	if err := os.Mkdir(filepath.Join(root, "one"), 0o755); err != nil {
		t.Fatal(err)
	}

	select {
	case event := <-rw.Events():
		if event.Path != target {
			t.Fatalf("event path = %q, want %q", event.Path, target)
		}
		if event.Op != uint32(fsnotify.Create) {
			t.Fatalf("event op = %d, want Create", event.Op)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for parent creation event")
	}
}

func TestReloadWatcherMapsParentEventsToMissingTarget(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "config")
	rw, err := NewReloadWatcher(context.Background(), []watchapi.Target{{ConfiguredPath: target}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := rw.Close(); err != nil {
			t.Errorf("close reload watcher: %v", err)
		}
	}()

	if err := os.WriteFile(target, []byte("config"), 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case event := <-rw.Events():
		if event.Path != target {
			t.Fatalf("event path = %q, want %q", event.Path, target)
		}
		if event.Op&uint32(fsnotify.Create) == 0 {
			t.Fatalf("event op = %d, want Create", event.Op)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for missing target event")
	}
}

func TestReloadWatcherDoesNotMapUnrelatedSiblingToMissingTarget(t *testing.T) {
	root := t.TempDir()
	targetA := filepath.Join(root, "a", "config")
	targetB := filepath.Join(root, "b", "config")
	rw, err := NewReloadWatcher(context.Background(), []watchapi.Target{
		{ConfiguredPath: targetA},
		{ConfiguredPath: targetB},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := rw.Close(); err != nil {
			t.Errorf("close reload watcher: %v", err)
		}
	}()

	if err := os.Mkdir(filepath.Join(root, "a"), 0o755); err != nil {
		t.Fatal(err)
	}
	select {
	case event := <-rw.Events():
		if event.Path != targetA {
			t.Fatalf("event path = %q, want %q", event.Path, targetA)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for target a parent creation event")
	}

	if err := os.Mkdir(filepath.Join(root, "unrelated"), 0o755); err != nil {
		t.Fatal(err)
	}
	select {
	case event := <-rw.Events():
		if event.Path == targetB {
			t.Fatalf("unrelated sibling event mapped to %q", targetB)
		}
	case <-time.After(300 * time.Millisecond):
	}
}

func TestReloadWatcherRearmsNestedParentAfterIntermediateCreate(t *testing.T) {
	root := t.TempDir()
	one := filepath.Join(root, "one")
	target := filepath.Join(one, "two", "config")
	if err := os.Mkdir(one, 0o755); err != nil {
		t.Fatal(err)
	}

	rw, err := NewReloadWatcher(context.Background(), []watchapi.Target{{ConfiguredPath: target}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := rw.Close(); err != nil {
			t.Errorf("close reload watcher: %v", err)
		}
	}()

	if err := os.Mkdir(filepath.Join(one, "two"), 0o755); err != nil {
		t.Fatal(err)
	}
	select {
	case event := <-rw.Events():
		if event.Path != target || event.Op&uint32(fsnotify.Create) == 0 {
			t.Fatalf("intermediate event = %#v, want target Create event", event)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for intermediate parent event")
	}

	if err := os.WriteFile(target, []byte("config"), 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case event := <-rw.Events():
		if event.Path != target || event.Op&uint32(fsnotify.Create) == 0 {
			t.Fatalf("target event = %#v, want target Create event", event)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for rearmed target event")
	}
}

func TestReloadWatcherMapsAncestorRemoveToTargetAndRearms(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "parent")
	target := filepath.Join(parent, "config")
	if err := os.Mkdir(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("config"), 0o600); err != nil {
		t.Fatal(err)
	}

	rw, err := NewReloadWatcher(context.Background(), []watchapi.Target{{ConfiguredPath: target}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := rw.Close(); err != nil {
			t.Errorf("close reload watcher: %v", err)
		}
	}()

	if err := os.RemoveAll(parent); err != nil {
		t.Fatal(err)
	}
	deadline := time.After(2 * time.Second)
	for {
		select {
		case event := <-rw.Events():
			if event.Path == target && event.Op&uint32(fsnotify.Remove) != 0 {
				goto removed
			}
		case <-deadline:
			t.Fatal("timed out waiting for target Remove event after ancestor removal")
		}
	}

removed:
	if err := os.Mkdir(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	deadline = time.After(2 * time.Second)
	for {
		select {
		case event := <-rw.Events():
			if event.Path == target && event.Op&uint32(fsnotify.Create) != 0 {
				if err := os.WriteFile(target, []byte("recreated"), 0o600); err != nil {
					t.Fatal(err)
				}
				return
			}
		case <-deadline:
			t.Fatal("timed out waiting for rearmed target after ancestor removal")
		}
	}
}
