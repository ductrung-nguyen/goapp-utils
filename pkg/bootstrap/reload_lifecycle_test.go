package bootstrap

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/ductrung-nguyen/goapp-utils/pkg/watchapi"
	"github.com/fsnotify/fsnotify"
)

type lifecycleSource struct {
	events chan watchapi.Event
	errors chan error
	mu     sync.Mutex
	closed bool
}

func newLifecycleSource() *lifecycleSource {
	return &lifecycleSource{events: make(chan watchapi.Event, 8), errors: make(chan error, 2)}
}
func (s *lifecycleSource) Add([]watchapi.Target) error   { return nil }
func (s *lifecycleSource) Events() <-chan watchapi.Event { return s.events }
func (s *lifecycleSource) Errors() <-chan error          { return s.errors }
func (s *lifecycleSource) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed {
		s.closed = true
		close(s.events)
		close(s.errors)
	}
	return nil
}

type blockingLifecycleSource struct {
	events       chan watchapi.Event
	errors       chan error
	closeStarted chan struct{}
	closeRelease chan struct{}
}

func (s *blockingLifecycleSource) Add([]watchapi.Target) error   { return nil }
func (s *blockingLifecycleSource) Events() <-chan watchapi.Event { return s.events }
func (s *blockingLifecycleSource) Errors() <-chan error          { return s.errors }
func (s *blockingLifecycleSource) Close() error {
	close(s.closeStarted)
	<-s.closeRelease
	return nil
}

func TestReloadManagerCloseContextUnblocksBlockedCallback(t *testing.T) {
	reg := lifecycleRegistration(1, func(RegistrationMetadata) (ReloadDecoder[int], error) { return nil, nil })
	callbackStarted := make(chan struct{})
	callbackRelease := make(chan struct{})
	m, err := NewReloadManager(context.Background(), reg, ReloadOptions[int]{Clone: identity[int], Subscribers: []Subscriber[int]{func(int, ReloadEvent[int]) {
		close(callbackStarted)
		<-callbackRelease
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Start(); err != nil {
		t.Fatal(err)
	}
	<-callbackStarted
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	if err := m.Close(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Close error = %v, want deadline", err)
	}
	close(callbackRelease)
	if err := m.Close(); err != nil {
		t.Fatalf("Close after callback release = %v", err)
	}
}

func TestReloadManagerCloseContextUnblocksBlockedSource(t *testing.T) {
	reg := lifecycleRegistration(1, func(RegistrationMetadata) (ReloadDecoder[int], error) { return nil, nil })
	src := &blockingLifecycleSource{events: make(chan watchapi.Event), errors: make(chan error), closeStarted: make(chan struct{}), closeRelease: make(chan struct{})}
	m, err := NewReloadManager(context.Background(), reg, ReloadOptions[int]{Clone: identity[int], Factory: func(context.Context, []watchapi.Target) (watchapi.Source, error) { return src, nil }})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Start(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	closeResult := make(chan error, 1)
	go func() { closeResult <- m.Close(ctx) }()
	<-src.closeStarted
	if err := <-closeResult; !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Close error = %v, want deadline", err)
	}
	close(src.closeRelease)
	if err := m.Close(); err != nil {
		t.Fatalf("Close after source release = %v", err)
	}
}

func lifecycleRegistration[T any](v T, decoder ReloadDecoderFactory[T]) *ReloadRegistration[T] {
	return &ReloadRegistration[T]{initial: v, selected: "config.yaml", metadata: RegistrationMetadata{owner: &Manager{}}, factory: decoder, zero: func() T { var z T; return z }}
}
func identity[T any](v T) T { return v }

func TestReloadManagerRejectsNegativeOptions(t *testing.T) {
	reg := lifecycleRegistration(1, func(RegistrationMetadata) (ReloadDecoder[int], error) { return nil, nil })
	for name, opts := range map[string]ReloadOptions[int]{
		"queue": {Clone: identity[int], QueueCapacity: -1}, "debounce": {Clone: identity[int], Stability: StabilityOptions{Debounce: -time.Millisecond}},
		"poll": {Clone: identity[int], Stability: StabilityOptions{PollInterval: -time.Millisecond}}, "retry": {Clone: identity[int], Stability: StabilityOptions{RetryDelay: -time.Millisecond}},
		"attempts": {Clone: identity[int], Stability: StabilityOptions{MaxAttempts: -1}}, "deleted": {Clone: identity[int], Deleted: DeletedPolicy(99)},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := NewReloadManager(context.Background(), reg, opts); !errors.Is(err, ErrInvalidOptions) {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestReloadManagerInitialValidation(t *testing.T) {
	reg := lifecycleRegistration(7, func(RegistrationMetadata) (ReloadDecoder[int], error) { return nil, nil })
	want := errors.New("invalid initial")
	_, err := NewReloadManager(context.Background(), reg, ReloadOptions[int]{Clone: identity[int], Validate: func(int) error { return want }})
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}

func TestReloadManagerStartPublishesInitialAndClones(t *testing.T) {
	reg := lifecycleRegistration([]int{1}, func(RegistrationMetadata) (ReloadDecoder[[]int], error) { return nil, nil })
	src := newLifecycleSource()
	got := make(chan []int, 1)
	m, err := NewReloadManager(context.Background(), reg, ReloadOptions[[]int]{Clone: func(v []int) []int { return append([]int(nil), v...) }, Factory: func(context.Context, []watchapi.Target) (watchapi.Source, error) { return src, nil }, Subscribers: []Subscriber[[]int]{func(v []int, ev ReloadEvent[[]int]) { got <- v; v[0] = 9; ev.Value[0] = 8 }}})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Start(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-got:
	case <-time.After(time.Second):
		t.Fatal("initial timeout")
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	v, ok := reg.Snapshot()
	if !ok || v[0] != 1 {
		t.Fatalf("registration mutated: %v, %v", v, ok)
	}
}

func TestReloadManagerStartInitialMissing(t *testing.T) {
	reg := lifecycleRegistration(0, func(RegistrationMetadata) (ReloadDecoder[int], error) { return nil, nil })
	reg.missing = true
	seen := make(chan ReloadKind, 1)
	m, err := NewReloadManager(context.Background(), reg, ReloadOptions[int]{Clone: identity[int], Subscribers: []Subscriber[int]{func(_ int, ev ReloadEvent[int]) { seen <- ev.Kind }}})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Start(); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-seen:
		if got != InitialMissing {
			t.Fatalf("event = %v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("initial timeout")
	}
	_ = m.Close()
}

func TestReloadManagerSubscriptionReentrancy(t *testing.T) {
	reg := lifecycleRegistration(1, func(RegistrationMetadata) (ReloadDecoder[int], error) { return nil, nil })
	var m *ReloadManager[int]
	result := make(chan error, 1)
	m, _ = NewReloadManager(context.Background(), reg, ReloadOptions[int]{Clone: identity[int], Subscribers: []Subscriber[int]{func(int, ReloadEvent[int]) { _, err := m.Subscribe(func(int, ReloadEvent[int]) {}); result <- err }}})
	if err := m.Start(); err != nil {
		t.Fatal(err)
	}
	err := <-result
	if !errors.Is(err, ErrReentrant) {
		t.Fatalf("error = %v", err)
	}
	_ = m.Close()
}

func TestReloadManagerQueueOverflowRetainsControlEvents(t *testing.T) {
	reg := lifecycleRegistration(1, func(RegistrationMetadata) (ReloadDecoder[int], error) { return nil, nil })
	m, err := NewReloadManager(context.Background(), reg, ReloadOptions[int]{Clone: identity[int], QueueCapacity: 2})
	if err != nil {
		t.Fatal(err)
	}
	m.enqueue(ReloadEvent[int]{Kind: Updated, Path: "a"})
	m.enqueue(ReloadEvent[int]{Kind: Updated, Path: "b"})
	m.enqueue(ReloadEvent[int]{Kind: Failed})
	m.enqueue(ReloadEvent[int]{Kind: Terminal})
	found := false
	for len(m.queue) > 0 {
		ev := <-m.queue
		found = found || ev.Kind == Failed || ev.Kind == Terminal
	}
	if !found && len(m.pending) == 0 {
		t.Fatal("control event discarded")
	}
}

func TestReloadManagerClosedLoop(t *testing.T) {
	reg := lifecycleRegistration(1, func(RegistrationMetadata) (ReloadDecoder[int], error) { return nil, nil })
	src := newLifecycleSource()
	m, err := NewReloadManager(context.Background(), reg, ReloadOptions[int]{Clone: identity[int], Factory: func(context.Context, []watchapi.Target) (watchapi.Source, error) { return src, nil }})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Start(); err != nil {
		t.Fatal(err)
	}
	_ = src.Close()
	if err := m.Wait(); err != nil {
		t.Fatal(err)
	}
}

func TestReloadManagerEventMasks(t *testing.T) {
	reg := lifecycleRegistration(1, func(RegistrationMetadata) (ReloadDecoder[int], error) {
		return func(io.Reader, string, RegistrationMetadata) (int, error) { return 2, nil }, nil
	})
	m, err := NewReloadManager(context.Background(), reg, ReloadOptions[int]{Clone: identity[int]})
	if err != nil {
		t.Fatal(err)
	}
	m.handleWatch(watchapi.Event{Op: uint32(fsnotify.Chmod), Path: "none"})
	if len(m.queue) != 0 {
		t.Fatal("chmod enqueued event")
	}
}

func TestReloadManagerPerSubscriberEventValueCloning(t *testing.T) {
	reg := lifecycleRegistration([]int{1}, func(RegistrationMetadata) (ReloadDecoder[[]int], error) { return nil, nil })
	second := make(chan []int, 1)
	m, err := NewReloadManager(context.Background(), reg, ReloadOptions[[]int]{Clone: func(v []int) []int { return append([]int(nil), v...) }, Subscribers: []Subscriber[[]int]{func(v []int, ev ReloadEvent[[]int]) { v[0] = 8; ev.Value[0] = 9 }, func(v []int, ev ReloadEvent[[]int]) { second <- append(v, ev.Value...) }}})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Start(); err != nil {
		t.Fatal(err)
	}
	got := <-second
	_ = m.Close()
	if len(got) != 2 || got[0] != 1 || got[1] != 1 {
		t.Fatalf("mutation leaked: %v", got)
	}
}

func TestReloadManagerReloadWriteEvent(t *testing.T) {
	path := t.TempDir() + "/config"
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	reg := lifecycleRegistration(1, func(RegistrationMetadata) (ReloadDecoder[int], error) {
		return func(io.Reader, string, RegistrationMetadata) (int, error) { return 2, nil }, nil
	})
	m, err := NewReloadManager(context.Background(), reg, ReloadOptions[int]{Clone: identity[int]})
	if err != nil {
		t.Fatal(err)
	}
	m.handleWatch(watchapi.Event{Op: uint32(fsnotify.Write), Path: path})
	if len(m.queue) != 1 || (<-m.queue).Kind != Updated {
		t.Fatal("write did not update")
	}
}
func TestReloadManagerClosedSubscriptionSkipsQueuedDelivery(t *testing.T) {
	reg := lifecycleRegistration(1, func(RegistrationMetadata) (ReloadDecoder[int], error) { return nil, nil })
	var calls int
	m, err := NewReloadManager(context.Background(), reg, ReloadOptions[int]{Clone: identity[int]})
	if err != nil {
		t.Fatal(err)
	}
	sub, err := m.Subscribe(func(int, ReloadEvent[int]) { calls++ })
	if err != nil {
		t.Fatal(err)
	}
	m.enqueue(ReloadEvent[int]{Kind: Updated, Value: 2, HasValue: true})
	if err := sub.Close(); err != nil {
		t.Fatal(err)
	}
	m.deliver(<-m.queue)
	if calls != 0 {
		t.Fatalf("closed subscription callback count = %d", calls)
	}
}

func TestReloadManagerSubscriptionCloseWaitsForActiveCallback(t *testing.T) {
	reg := lifecycleRegistration(1, func(RegistrationMetadata) (ReloadDecoder[int], error) { return nil, nil })
	entered := make(chan struct{})
	release := make(chan struct{})
	m, err := NewReloadManager(context.Background(), reg, ReloadOptions[int]{Clone: identity[int]})
	if err != nil {
		t.Fatal(err)
	}
	sub, err := m.Subscribe(func(int, ReloadEvent[int]) {
		close(entered)
		<-release
	})
	if err != nil {
		t.Fatal(err)
	}
	delivered := make(chan struct{})
	go func() {
		m.deliver(ReloadEvent[int]{Kind: Updated, Value: 2, HasValue: true})
		close(delivered)
	}()
	<-entered
	closed := make(chan error, 1)
	go func() { closed <- sub.Close() }()
	select {
	case err := <-closed:
		t.Fatalf("Close returned before callback finished: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	select {
	case err := <-closed:
		if err != nil {
			t.Fatalf("Close error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Close did not wait for callback")
	}
	<-delivered
}

func TestReloadManagerSubscriptionCloseFromCallbackIsReentrant(t *testing.T) {
	reg := lifecycleRegistration(1, func(RegistrationMetadata) (ReloadDecoder[int], error) { return nil, nil })
	var sub *Subscription
	result := make(chan error, 1)
	m, err := NewReloadManager(context.Background(), reg, ReloadOptions[int]{Clone: identity[int]})
	if err != nil {
		t.Fatal(err)
	}
	sub, err = m.Subscribe(func(_ int, ev ReloadEvent[int]) { result <- sub.Close(ev.Context) })
	if err != nil {
		t.Fatal(err)
	}
	m.deliver(ReloadEvent[int]{Kind: Updated})
	if err := <-result; !errors.Is(err, ErrReentrant) {
		t.Fatalf("Close error = %v", err)
	}
}

func TestReloadManagerMissingExplicitTargetWatchesParent(t *testing.T) {
	reg := lifecycleRegistration(0, func(RegistrationMetadata) (ReloadDecoder[int], error) {
		return func(io.Reader, string, RegistrationMetadata) (int, error) { return 1, nil }, nil
	})
	reg.selected = filepath.Join(t.TempDir(), "nested", "config.yaml")
	reg.missing = true
	source := newLifecycleSource()
	var got []watchapi.Target
	rm, err := NewReloadManager(context.Background(), reg, ReloadOptions[int]{
		Clone: identity[int],
		Factory: func(_ context.Context, targets []watchapi.Target) (watchapi.Source, error) {
			got = append([]watchapi.Target(nil), targets...)
			return source, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := rm.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := rm.Close(); err != nil {
			t.Errorf("close reload manager: %v", err)
		}
	}()
	if len(got) != 1 || got[0].ConfiguredPath != reg.selected || got[0].ParentPath != filepath.Dir(reg.selected) {
		t.Fatalf("watch target = %#v, want configured file and parent directory", got)
	}
}

func TestReloadManagerRejectsPostOpenSymlinkSwap(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "inside.yaml")
	outside := filepath.Join(t.TempDir(), "outside.yaml")
	path := filepath.Join(root, "config.yaml")
	if err := os.WriteFile(inside, []byte("inside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outside, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(inside, path); err != nil {
		t.Fatal(err)
	}

	reg := lifecycleRegistration(0, func(RegistrationMetadata) (ReloadDecoder[int], error) {
		return func(io.Reader, string, RegistrationMetadata) (int, error) { return 1, nil }, nil
	})
	reg.selected = path
	m, err := NewReloadManager(context.Background(), reg, ReloadOptions[int]{
		Clone:     identity[int],
		Stability: StabilityOptions{MaxAttempts: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	swapped := false
	m.postOpenHook = func(name string) {
		if swapped || name != path {
			return
		}
		swapped = true
		if err := os.Remove(path); err != nil {
			t.Fatalf("remove symlink: %v", err)
		}
		if err := os.Symlink(outside, path); err != nil {
			t.Fatalf("replace symlink: %v", err)
		}
	}

	_, err = m.loadCandidateRetry(path)
	if err == nil {
		t.Fatal("post-open symlink swap was accepted")
	}
	if !swapped {
		t.Fatalf("post-open hook was not called (err=%v)", err)
	}
}

func TestReloadManagerRetryRecoversWithinAttemptLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("config"), 0o600); err != nil {
		t.Fatal(err)
	}
	const failures = 2
	attempts := 0
	reg := lifecycleRegistration(0, func(RegistrationMetadata) (ReloadDecoder[int], error) {
		return func(io.Reader, string, RegistrationMetadata) (int, error) {
			attempts++
			if attempts <= failures {
				return 0, errors.New("transient decode failure")
			}
			return 7, nil
		}, nil
	})
	reg.selected = path
	m, err := NewReloadManager(context.Background(), reg, ReloadOptions[int]{
		Clone:     identity[int],
		Stability: StabilityOptions{MaxAttempts: failures + 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := m.loadCandidateRetry(path)
	if err != nil {
		t.Fatalf("loadCandidateRetry error = %v", err)
	}
	if got != 7 || attempts != failures+1 {
		t.Fatalf("result = %d after %d attempts, want 7 after %d", got, attempts, failures+1)
	}
}

func TestReloadManagerRetryHonorsAttemptLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("config"), 0o600); err != nil {
		t.Fatal(err)
	}
	attempts := 0
	want := errors.New("persistent decode failure")
	reg := lifecycleRegistration(0, func(RegistrationMetadata) (ReloadDecoder[int], error) {
		return func(io.Reader, string, RegistrationMetadata) (int, error) {
			attempts++
			return 0, want
		}, nil
	})
	reg.selected = path
	const maxAttempts = 2
	m, err := NewReloadManager(context.Background(), reg, ReloadOptions[int]{
		Clone:     identity[int],
		Stability: StabilityOptions{MaxAttempts: maxAttempts},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = m.loadCandidateRetry(path)
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
	if attempts != maxAttempts {
		t.Fatalf("attempts = %d, want %d", attempts, maxAttempts)
	}
}

func TestReloadManagerFollowCandidateSwitchOwnsMutex(t *testing.T) {
	root := t.TempDir()
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	oldPath := filepath.Join(root, "old.yaml")
	newPath := filepath.Join(root, "new.yaml")
	if err := os.WriteFile(newPath, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	decoder := func(RegistrationMetadata) (ReloadDecoder[string], error) {
		return func(_ io.Reader, path string, _ RegistrationMetadata) (string, error) {
			return filepath.Base(path), nil
		}, nil
	}

	t.Run("candidate switch", func(t *testing.T) {
		reg := lifecycleRegistration("old", decoder)
		reg.selected = oldPath
		reg.candidates = []string{oldPath, newPath}
		m, err := NewReloadManager(context.Background(), reg, ReloadOptions[string]{Clone: identity[string]})
		if err != nil {
			t.Fatal(err)
		}
		if !m.followCandidate(oldPath) {
			var ev ReloadEvent[string]
			select {
			case ev = <-m.queue:
			default:
			}
			t.Fatalf("followCandidate did not switch; event=%#v", ev)
		}
		m.mu.Lock()
		selected, value, path := m.reg.selected, m.value, m.path
		m.mu.Unlock()
		if selected != newPath || value != "new.yaml" || path != newPath {
			t.Fatalf("state = selected %q, value %q, path %q", selected, value, path)
		}
		select {
		case ev := <-m.queue:
			if ev.Kind != Deleted || ev.Path != oldPath {
				t.Fatalf("event = %#v, want Deleted for old candidate", ev)
			}
		default:
			t.Fatal("candidate switch did not publish deletion event")
		}
	})

	t.Run("initial missing creation", func(t *testing.T) {
		missingPath := filepath.Join(root, "missing.yaml")
		reg := lifecycleRegistration("", decoder)
		reg.selected = missingPath
		reg.missing = true
		reg.candidates = []string{missingPath, newPath}
		m, err := NewReloadManager(context.Background(), reg, ReloadOptions[string]{Clone: identity[string]})
		if err != nil {
			t.Fatal(err)
		}
		if !m.followCandidate("") {
			t.Fatal("followCandidate did not create initial value")
		}
		m.mu.Lock()
		selected, missing, value, path := m.reg.selected, m.reg.missing, m.value, m.path
		m.mu.Unlock()
		if selected != newPath || missing || value != "new.yaml" || path != newPath {
			t.Fatalf("state = selected %q, missing %v, value %q, path %q", selected, missing, value, path)
		}
	})
}
