package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ductrung-nguyen/goapp-utils/pkg/filewatcher"
	"github.com/ductrung-nguyen/goapp-utils/pkg/watchapi"
	"github.com/fsnotify/fsnotify"
)

type DeletedPolicy uint8

const (
	RetainLastGood DeletedPolicy = iota
	Clear
	Stop
)

type StabilityOptions struct {
	Debounce, PollInterval time.Duration
	MaxAttempts            int
	RetryDelay             time.Duration
}

type ReloadKind int

const (
	Initial ReloadKind = iota
	InitialMissing
	Updated
	Deleted
	Failed
	Terminal
)

type ReloadEvent[T any] struct {
	Kind     ReloadKind
	Value    T
	HasValue bool
	Path     string
	Err      error
	Context  context.Context
}

type Subscriber[T any] func(T, ReloadEvent[T])

var (
	ErrReentrant      = errors.New("reload manager method called from callback")
	ErrTerminal       = errors.New("reload manager terminated")
	ErrClosed         = errors.New("reload manager closed")
	ErrInvalidOptions = errors.New("reload manager invalid options")
	ErrCallbackPanic  = errors.New("reload callback panicked")
)

type TerminalError struct {
	Cause error
	Path  string
}

func (e *TerminalError) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause == nil {
		return "reload manager terminated"
	}
	return fmt.Sprintf("reload manager terminal: %v", e.Cause)
}
func (e *TerminalError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

type ReloadOptions[T any] struct {
	Deleted       DeletedPolicy
	FollowSearch  bool
	Stability     StabilityOptions
	QueueCapacity int
	Factory       watchapi.Factory
	Clone         func(T) T
	Validate      func(T) error
	OnError       func(ReloadEvent[T])
	Subscribers   []Subscriber[T]
}

type reloadState uint8

const (
	reloadConstructing reloadState = iota
	reloadRunning
	reloadTerminal
	reloadClosed
)

type reloadSub[T any] struct {
	fn     Subscriber[T]
	mu     sync.Mutex
	closed bool
	active int
	wait   chan struct{}
}
type Subscription struct{ close func(context.Context) error }

func (s *Subscription) Close(ctx ...context.Context) error {
	if s == nil || s.close == nil {
		return nil
	}
	var callbackCtx context.Context
	if len(ctx) > 0 {
		callbackCtx = ctx[0]
	}
	return s.close(callbackCtx)
}

type ReloadManager[T any] struct {
	mu              sync.Mutex
	parent          context.Context
	reg             *ReloadRegistration[T]
	opts            ReloadOptions[T]
	state           reloadState
	value           T
	hasValue        bool
	path            string
	source          watchapi.Source
	postOpenHook    func(string)
	queue           chan ReloadEvent[T]
	done            chan struct{}
	terminalErr     error
	subs            []*reloadSub[T]
	pending         []ReloadEvent[T]
	wg              sync.WaitGroup
	dispatchMu      sync.Mutex
	callbackDepth   int
	callbackSub     *reloadSub[T]
	callbackOwner   *callbackToken
	callbackIdle    chan struct{}
	sourceCloseDone chan struct{}
	sourceCloseErr  error
}

func NewReloadManager[T any](parent context.Context, registration *ReloadRegistration[T], opts ReloadOptions[T]) (*ReloadManager[T], error) {
	if parent == nil {
		return nil, ErrInvalidOptions
	}
	if registration == nil || registration.metadata.owner == nil || registration.factory == nil {
		return nil, ErrInvalidRegistration
	}
	if opts.Clone == nil {
		return nil, ErrInvalidOptions
	}
	if opts.Factory == nil {
		opts.Factory = filewatcher.Factory
	}
	if opts.Deleted > Stop || opts.QueueCapacity < 0 || opts.Stability.Debounce < 0 || opts.Stability.PollInterval < 0 || opts.Stability.RetryDelay < 0 || opts.Stability.MaxAttempts < 0 {
		return nil, ErrInvalidOptions
	}
	if opts.QueueCapacity == 0 {
		opts.QueueCapacity = 16
	}
	if opts.Stability.MaxAttempts == 0 {
		opts.Stability.MaxAttempts = 3
	}
	if opts.Stability.RetryDelay == 0 {
		opts.Stability.RetryDelay = 10 * time.Millisecond
	}
	r := &ReloadManager[T]{parent: parent, reg: registration, opts: opts, state: reloadConstructing, queue: make(chan ReloadEvent[T], opts.QueueCapacity), done: make(chan struct{}), callbackIdle: make(chan struct{})}
	close(r.callbackIdle)
	if registration.missing {
		r.value = opts.Clone(registration.missingValue)
		r.hasValue = false
	} else {
		r.value = opts.Clone(registration.initial)
		r.hasValue = true
	}
	for _, fn := range opts.Subscribers {
		if fn != nil {
			r.subs = append(r.subs, &reloadSub[T]{fn: fn, wait: make(chan struct{})})
		}
	}
	if opts.Validate != nil && r.hasValue {
		if err := opts.Validate(r.value); err != nil {
			return nil, err
		}
	}
	return r, nil
}

func (r *ReloadManager[T]) Start() error {
	r.mu.Lock()
	if r.callbackDepth > 0 {
		r.mu.Unlock()
		return ErrReentrant
	}
	if r.state == reloadClosed {
		r.mu.Unlock()
		return ErrClosed
	}
	if r.state == reloadTerminal {
		r.mu.Unlock()
		return ErrTerminal
	}
	if r.state == reloadRunning {
		r.mu.Unlock()
		return nil
	}
	var src watchapi.Source
	if r.opts.Factory != nil {
		targets := []watchapi.Target{}
		if r.reg.selected == "" && len(r.reg.candidates) > 0 {
			r.reg.selected = r.reg.candidates[0]
		}
		if r.reg.selected != "" {
			parentPath := r.reg.selected
			if r.reg.missing {
				parentPath = filepath.Dir(parentPath)
			}
			targets = append(targets, watchapi.Target{ConfiguredPath: r.reg.selected, ParentPath: parentPath})
		}
		var err error
		src, err = r.opts.Factory(r.parent, targets)
		if err != nil {
			if src != nil {
				_ = src.Close()
			}
			r.state = reloadTerminal
			r.terminalErr = err
			r.mu.Unlock()
			r.enqueue(ReloadEvent[T]{Kind: Terminal, Err: &TerminalError{Cause: err}, Path: r.reg.selected})
			r.wg.Add(1)
			go r.loop()
			return err
		}
		r.source = src
	}
	r.state = reloadRunning
	ev := ReloadEvent[T]{Kind: Initial, Value: r.value, HasValue: r.hasValue, Path: r.reg.selected}
	if r.reg.missing {
		ev.Kind = InitialMissing
	}
	// Publish the initial event before the dispatcher can observe an empty source.
	r.queue <- ev
	r.wg.Add(1)
	go r.loop()
	r.mu.Unlock()
	return nil
}

func (r *ReloadManager[T]) loop() {
	defer r.wg.Done()
	ctx, cancel := context.WithCancel(r.parent)
	defer cancel()
	r.mu.Lock()
	src := r.source
	r.mu.Unlock()
	var evch <-chan watchapi.Event
	var errch <-chan error
	if src != nil {
		evch, errch = src.Events(), src.Errors()
	}
	for {
		if len(r.queue) > 0 {
			ev := <-r.queue
			r.deliver(ev)
			continue
		}
		r.mu.Lock()
		terminal, closed := r.state == reloadTerminal, r.state == reloadClosed
		var pending *ReloadEvent[T]
		if len(r.pending) > 0 {
			ev := r.pending[0]
			r.pending = r.pending[1:]
			pending = &ev
		}
		r.mu.Unlock()
		if pending != nil {
			r.deliver(*pending)
			continue
		}
		if closed || evch == nil && errch == nil || terminal && len(r.queue) == 0 {
			return
		}
		select {
		case <-ctx.Done():
			r.finish(ctx.Err())
		case <-r.done:
			return
		case ev, ok := <-evch:
			if !ok {
				evch = nil
			} else {
				r.handleWatch(ev)
			}
		case err, ok := <-errch:
			if !ok {
				errch = nil
			} else if err != nil {
				r.finish(err)
			}
		case ev := <-r.queue:
			r.deliver(ev)
		}
	}
}
func (r *ReloadManager[T]) enqueue(ev ReloadEvent[T]) {
	select {
	case r.queue <- ev:
	default:
		r.mu.Lock()
		r.coalesceLocked(ev)
		r.mu.Unlock()
	}
}
func (r *ReloadManager[T]) coalesceLocked(ev ReloadEvent[T]) {
	if ev.Kind == Terminal {
		kept := make([]ReloadEvent[T], 0, cap(r.queue))
		for len(r.queue) > 0 {
			x := <-r.queue
			switch x.Kind {
			case Initial, InitialMissing, Failed, Terminal:
				kept = append(kept, x)
			}
		}
		if len(kept) < cap(r.queue) {
			kept = append(kept, ev)
			for _, x := range kept {
				r.queue <- x
			}
			return
		}
		for _, x := range kept {
			r.queue <- x
		}
		r.pending = append(r.pending, ev)
		return
	}
	n := len(r.queue)
	tmp := make([]ReloadEvent[T], 0, n)
	replaced := false
	for i := 0; i < n; i++ {
		x := <-r.queue
		if ev.Kind == Updated && x.Kind == Updated && x.Path == ev.Path {
			if !replaced {
				tmp = append(tmp, ev)
				replaced = true
			}
			continue
		}
		tmp = append(tmp, x)
	}
	if !replaced {
		if len(tmp) >= cap(r.queue) {
			victim := -1
			for i, x := range tmp {
				if x.Kind == Updated {
					victim = i
					break
				}
			}
			if victim >= 0 {
				tmp = append(tmp[:victim], tmp[victim+1:]...)
			} else {
				r.pending = append(r.pending, ev)
				for _, x := range tmp {
					r.queue <- x
				}
				return
			}
		}
		tmp = append(tmp, ev)
	}
	for _, x := range tmp {
		r.queue <- x
	}
}

func (r *ReloadManager[T]) handleWatch(w watchapi.Event) {
	if w.Err != nil {
		r.enqueue(ReloadEvent[T]{Kind: Failed, Path: w.Path, Err: w.Err})
		return
	}
	const (
		write  = uint32(fsnotify.Write | fsnotify.Create)
		remove = uint32(fsnotify.Remove)
		rename = uint32(fsnotify.Rename)
	)
	if w.Op&remove != 0 {
		r.deleted(w.Path)
		return
	}
	if w.Op&(write|rename) != 0 {
		r.reload(w.Path)
	}
}
func (r *ReloadManager[T]) reload(path string) {
	if r.opts.FollowSearch && r.followCandidate(path) {
		return
	}
	if d := r.opts.Stability.Debounce; d > 0 {
		time.Sleep(d)
	}
	v, err := r.loadCandidateRetry(path)
	if err != nil {
		r.enqueue(ReloadEvent[T]{Kind: Failed, Path: path, Err: err})
		return
	}
	r.mu.Lock()
	r.value, r.hasValue, r.path = v, true, path
	r.mu.Unlock()
	r.enqueue(ReloadEvent[T]{Kind: Updated, Value: r.opts.Clone(v), HasValue: true, Path: path})
}

func (r *ReloadManager[T]) loadCandidateRetry(path string) (T, error) {
	var zero T
	var err error
	for attempt := 0; attempt < r.opts.Stability.MaxAttempts; attempt++ {
		if attempt > 0 {
			if err := r.waitStability(); err != nil {
				return zero, err
			}
		}
		var value T
		value, err = r.loadCandidate(path)
		if err == nil {
			return value, nil
		}
	}
	return zero, err
}

func (r *ReloadManager[T]) waitStability() error {
	for _, d := range []time.Duration{r.opts.Stability.RetryDelay, r.opts.Stability.PollInterval} {
		if d <= 0 {
			continue
		}
		t := time.NewTimer(d)
		select {
		case <-t.C:
		case <-r.parent.Done():
			if !t.Stop() {
				<-t.C
			}
			return r.parent.Err()
		}
	}
	return nil
}

func (r *ReloadManager[T]) loadCandidate(path string) (T, error) {
	var zero T
	if err := r.candidateContained(path); err != nil {
		return zero, err
	}
	f, err := os.Open(path)
	if err != nil {
		return zero, err
	}
	defer func() { _ = f.Close() }()
	opened, err := f.Stat()
	if err != nil {
		return zero, err
	}
	if r.postOpenHook != nil {
		r.postOpenHook(path)
	}
	if err != nil {
		return zero, err
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return zero, err
	}
	current, err := os.Stat(resolved)
	if err != nil {
		return zero, err
	}
	if !os.SameFile(opened, current) {
		return zero, fmt.Errorf("candidate changed while opening: %s", path)
	}
	if err := r.candidateContainedResolved(resolved); err != nil {
		return zero, err
	}
	dec, err := r.reg.factory(r.reg.metadata)
	if err != nil {
		return zero, err
	}
	v, err := dec(f, path, r.reg.metadata)
	if err != nil {
		return zero, err
	}
	if r.opts.Validate != nil {
		if err := r.opts.Validate(v); err != nil {
			return zero, err
		}
	}
	return r.opts.Clone(v), nil
}

func (r *ReloadManager[T]) candidateContained(path string) error {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	return r.candidateContainedResolved(resolved)
}

func (r *ReloadManager[T]) candidateContainedResolved(path string) error {
	if len(r.reg.candidates) == 0 {
		return nil
	}
	for _, candidate := range r.reg.candidates {
		root, err := filepath.Abs(filepath.Dir(candidate))
		if err == nil && contained(root, path) {
			return nil
		}
	}
	return &InvalidPathError{Path: path, Reason: "path is outside configured search roots"}
}

func (r *ReloadManager[T]) followCandidate(path string) bool {
	r.mu.Lock()
	selected, missing := r.reg.selected, r.reg.missing
	candidates := append([]string(nil), r.reg.candidates...)
	src := r.source
	r.mu.Unlock()
	if len(candidates) == 0 || (path != selected && (!missing || path != "")) {
		return false
	}
	switched := false
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err != nil {
			continue
		}
		v, err := r.loadCandidateRetry(candidate)
		if err != nil {
			// A present candidate is authoritative even when invalid; do not fall back.
			r.enqueue(ReloadEvent[T]{Kind: Failed, Path: candidate, Err: err})
			return true
		}
		if candidate == selected && !missing {
			return switched
		}
		if src != nil {
			parent := candidate
			if missing {
				parent = filepath.Dir(candidate)
			}
			_ = src.Add([]watchapi.Target{{ConfiguredPath: candidate, ParentPath: parent}})
		}
		r.mu.Lock()
		oldPath := r.reg.selected
		oldValue, oldHas := r.value, r.hasValue
		r.reg.selected, r.reg.missing = candidate, false
		r.value, r.hasValue, r.path = v, true, candidate
		r.mu.Unlock()
		switched = true
		if oldPath != "" {
			switch r.opts.Deleted {
			case Stop:
				r.finish(&TerminalError{Cause: ErrTerminal, Path: oldPath})
				return true
			case Clear:
				z := r.opts.Clone(r.reg.zero())
				r.enqueue(ReloadEvent[T]{Kind: Deleted, Value: z, HasValue: false, Path: oldPath})
			default:
				r.enqueue(ReloadEvent[T]{Kind: Deleted, Value: r.opts.Clone(oldValue), HasValue: oldHas, Path: oldPath})
			}
		}
	}
	return switched
}

func (r *ReloadManager[T]) deleted(path string) {
	if r.opts.FollowSearch && r.followCandidate(path) {
		return
	}
	r.mu.Lock()
	p, v, has := r.opts.Deleted, r.value, r.hasValue
	r.mu.Unlock()
	switch p {
	case RetainLastGood:
		r.enqueue(ReloadEvent[T]{Kind: Deleted, Value: r.opts.Clone(v), HasValue: has, Path: path})
	case Clear:
		z := r.opts.Clone(r.reg.zero())
		r.mu.Lock()
		r.value, r.hasValue = z, false
		r.mu.Unlock()
		r.enqueue(ReloadEvent[T]{Kind: Deleted, Value: r.opts.Clone(z), Path: path})
	case Stop:
		r.finish(&TerminalError{Cause: ErrTerminal, Path: path})
	}
}

func (r *ReloadManager[T]) deliver(ev ReloadEvent[T]) {
	r.dispatchMu.Lock()
	defer r.dispatchMu.Unlock()
	r.mu.Lock()
	if r.state == reloadClosed {
		r.mu.Unlock()
		return
	}
	subs := append([]*reloadSub[T](nil), r.subs...)
	onerr := r.opts.OnError
	r.mu.Unlock()
	if (ev.Kind == Failed || ev.Kind == Terminal) && onerr != nil {
		token := &callbackToken{manager: r}
		ev.Context = context.WithValue(r.parent, callbackTokenKey{}, token)
		r.mu.Lock()
		if r.callbackDepth == 0 {
			r.callbackIdle = make(chan struct{})
		}
		r.callbackDepth++
		r.callbackOwner = token
		r.mu.Unlock()
		pan := callError(onerr, ev)
		r.mu.Lock()
		r.callbackDepth--
		if r.callbackDepth == 0 {
			r.callbackOwner = nil
			close(r.callbackIdle)
		}
		r.mu.Unlock()
		if pan {
			r.finish(ErrCallbackPanic)
		}
	}
	for _, s := range subs {
		var val T
		callbackEvent := ev
		if ev.HasValue {
			val = r.opts.Clone(ev.Value)
			callbackEvent.Value = r.opts.Clone(ev.Value)
		}
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			continue
		}
		s.active++
		s.mu.Unlock()
		token := &callbackToken{manager: r, sub: s}
		callbackEvent.Context = context.WithValue(r.parent, callbackTokenKey{}, token)
		r.mu.Lock()
		if r.callbackDepth == 0 {
			r.callbackIdle = make(chan struct{})
		}
		r.callbackDepth++
		r.callbackSub = s
		r.callbackOwner = token
		r.mu.Unlock()
		pan := callSubscriber(s.fn, val, callbackEvent)
		r.mu.Lock()
		r.callbackDepth--
		if r.callbackDepth == 0 {
			r.callbackSub = nil
			r.callbackOwner = nil
			close(r.callbackIdle)
		}
		r.mu.Unlock()
		s.mu.Lock()
		s.active--
		if s.active == 0 {
			close(s.wait)
			s.wait = make(chan struct{})
		}
		s.mu.Unlock()
		if pan {
			r.enqueue(ReloadEvent[T]{Kind: Failed, Path: ev.Path, Err: ErrCallbackPanic})
		}
	}
}

type callbackTokenKey struct{}
type callbackToken struct {
	manager any
	sub     any
}

func callError[T any](fn func(ReloadEvent[T]), ev ReloadEvent[T]) (pan bool) {
	defer func() {
		if recover() != nil {
			pan = true
		}
	}()
	fn(ev)
	return
}
func callSubscriber[T any](fn Subscriber[T], v T, ev ReloadEvent[T]) (pan bool) {
	defer func() {
		if recover() != nil {
			pan = true
		}
	}()
	fn(v, ev)
	return
}

func (r *ReloadManager[T]) finish(cause error) {
	r.mu.Lock()
	if r.state == reloadClosed || r.state == reloadTerminal {
		r.mu.Unlock()
		return
	}
	r.state, r.terminalErr = reloadTerminal, cause
	path, src := r.path, r.source
	r.source = nil
	r.mu.Unlock()
	r.enqueue(ReloadEvent[T]{Kind: Terminal, Err: &TerminalError{Cause: cause, Path: path}, Path: path})
	if src != nil {
		_ = src.Close()
	}
}
func (r *ReloadManager[T]) Subscribe(fn Subscriber[T]) (*Subscription, error) {
	if fn == nil {
		return nil, ErrInvalidOptions
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.callbackDepth > 0 {
		return nil, ErrReentrant
	}
	if r.state == reloadClosed {
		return nil, ErrClosed
	}
	if r.state == reloadTerminal {
		return nil, ErrTerminal
	}
	s := &reloadSub[T]{fn: fn, wait: make(chan struct{})}
	r.subs = append(r.subs, s)
	return &Subscription{close: func(ctx context.Context) error { return r.closeSub(s, ctx) }}, nil
}
func (r *ReloadManager[T]) closeSub(s *reloadSub[T], callbackCtx context.Context) error {
	r.mu.Lock()
	if callbackCtx != nil {
		if token, ok := callbackCtx.Value(callbackTokenKey{}).(*callbackToken); ok && token.manager == r && (token.sub == nil || token.sub == s) {
			r.mu.Unlock()
			return ErrReentrant
		}
	}
	r.mu.Unlock()

	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	if s.active > 0 {
		ch := s.wait
		s.mu.Unlock()
		if callbackCtx == nil {
			<-ch
			return nil
		}
		select {
		case <-ch:
			return nil
		case <-callbackCtx.Done():
			return callbackCtx.Err()
		}
	}
	s.mu.Unlock()
	return nil
}
func (r *ReloadManager[T]) Wait() error {
	r.mu.Lock()
	if r.callbackDepth > 0 {
		r.mu.Unlock()
		return ErrReentrant
	}
	r.mu.Unlock()
	r.wg.Wait()
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.terminalErr
}

func (r *ReloadManager[T]) Close(ctx ...context.Context) error {
	var callbackCtx context.Context
	if len(ctx) > 0 {
		callbackCtx = ctx[0]
	}
	r.mu.Lock()
	if callbackCtx != nil {
		if token, ok := callbackCtx.Value(callbackTokenKey{}).(*callbackToken); ok && token.manager == r {
			r.mu.Unlock()
			return ErrReentrant
		}
	}
	if r.state == reloadClosed {
		done := r.sourceCloseDone
		idle := r.callbackIdle
		r.mu.Unlock()
		waitDone := make(chan struct{})
		go func() { r.wg.Wait(); close(waitDone) }()
		if err := waitContext(waitDone, callbackCtx); err != nil {
			return err
		}
		if err := waitContext(idle, callbackCtx); err != nil {
			return err
		}
		if done != nil {
			if err := waitContext(done, callbackCtx); err != nil {
				return err
			}
			r.mu.Lock()
			err := r.sourceCloseErr
			r.mu.Unlock()
			return err
		}
		return nil
	}
	src := r.source
	r.source = nil
	r.state = reloadClosed
	close(r.done)
	var closeDone chan struct{}
	if src != nil {
		closeDone = make(chan struct{})
		r.sourceCloseDone = closeDone
	}
	r.mu.Unlock()
	if src != nil {
		go func() {
			err := src.Close()
			r.mu.Lock()
			r.sourceCloseErr = err
			r.mu.Unlock()
			close(closeDone)
		}()
	}
	waitDone := make(chan struct{})
	go func() { r.wg.Wait(); close(waitDone) }()
	if err := waitContext(waitDone, callbackCtx); err != nil {
		return err
	}
	r.mu.Lock()
	idle := r.callbackIdle
	err := r.terminalErr
	r.mu.Unlock()
	if err := waitContext(idle, callbackCtx); err != nil {
		return err
	}
	if closeDone != nil {
		if err := waitContext(closeDone, callbackCtx); err != nil {
			return err
		}
		r.mu.Lock()
		err = r.sourceCloseErr
		r.mu.Unlock()
	}
	return err
}

func waitContext(done <-chan struct{}, ctx context.Context) error {
	if ctx == nil {
		<-done
		return nil
	}
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
