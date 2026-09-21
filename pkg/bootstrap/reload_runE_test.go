package bootstrap

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/ductrung-nguyen/goapp-utils/pkg/vcflag"
	"github.com/ductrung-nguyen/goapp-utils/pkg/watchapi"
	"github.com/spf13/cobra"
)

type runETestSource struct {
	mu     sync.Mutex
	closed bool
	events chan watchapi.Event
	errors chan error
}

func (s *runETestSource) Add([]watchapi.Target) error   { return nil }
func (s *runETestSource) Events() <-chan watchapi.Event { return s.events }
func (s *runETestSource) Errors() <-chan error          { return s.errors }
func (s *runETestSource) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed {
		s.closed = true
		close(s.events)
		close(s.errors)
	}
	return nil
}

func (s *runETestSource) isClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

func TestNewReloadRunEValidatesOnlyStableInputs(t *testing.T) {
	manager, err := New(Options{CommandUse: "app"}, reloadConfig{})
	if err != nil {
		t.Fatal(err)
	}
	builder := func(vcflag.Metadata, PrototypeSchema, []byte) (ReloadDecoderFactory[reloadConfig], error) {
		return func(RegistrationMetadata) (ReloadDecoder[reloadConfig], error) { return nil, nil }, nil
	}
	invalid := ReloadOptions[reloadConfig]{}
	if _, err := NewReloadRunE[reloadConfig](manager, func() reloadConfig { return reloadConfig{} }, builder, invalid); err != nil {
		t.Fatalf("constructor rejected runtime options: %v", err)
	}
	if _, err := NewReloadRunE[reloadConfig](nil, func() reloadConfig { return reloadConfig{} }, builder, ReloadOptions[reloadConfig]{Clone: func(v reloadConfig) reloadConfig { return v }}); err == nil {
		t.Fatal("nil manager accepted")
	}
	if _, err := NewReloadRunE[reloadConfig](manager, nil, builder, ReloadOptions[reloadConfig]{Clone: func(v reloadConfig) reloadConfig { return v }}); err == nil {
		t.Fatal("nil zero accepted")
	}
	if _, err := NewReloadRunE[reloadConfig](manager, func() reloadConfig { return reloadConfig{} }, nil, ReloadOptions[reloadConfig]{Clone: func(v reloadConfig) reloadConfig { return v }}); err == nil {
		t.Fatal("nil builder accepted")
	}
}

func TestNewReloadRunEBuildsAfterParseAndPreservesFactoryError(t *testing.T) {
	manager, err := New(Options{CommandUse: "app", MissingPolicy: MissingConfigAllowed}, reloadConfig{})
	if err != nil {
		t.Fatal(err)
	}
	cmd, err := manager.NewCommand()
	if err != nil {
		t.Fatal(err)
	}
	cmd.SetContext(context.Background())
	var calls int
	factoryErr := errors.New("factory failed")
	builder := func(raw vcflag.Metadata, _ PrototypeSchema, _ []byte) (ReloadDecoderFactory[reloadConfig], error) {
		calls++
		if len(raw.Fields()) == 0 {
			t.Fatal("builder ran before command metadata was available")
		}
		return func(RegistrationMetadata) (ReloadDecoder[reloadConfig], error) { return nil, nil }, nil
	}
	source := &runETestSource{events: make(chan watchapi.Event), errors: make(chan error)}
	opts := ReloadOptions[reloadConfig]{Clone: func(v reloadConfig) reloadConfig { return v }, Factory: func(context.Context, []watchapi.Target) (watchapi.Source, error) {
		return source, factoryErr
	}}
	runE, err := NewReloadRunE[reloadConfig](manager, func() reloadConfig { return reloadConfig{} }, builder, opts)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatal("builder called during construction")
	}
	if err := runE(cmd, nil); !errors.Is(err, factoryErr) {
		t.Fatalf("run error = %v, want factory error", err)
	}
	if calls != 1 {
		t.Fatalf("builder calls = %d, want 1", calls)
	}
	if !source.isClosed() {
		t.Fatal("factory source was not closed")
	}
}

func TestNewReloadRunERejectsForeignAndNilCommands(t *testing.T) {
	manager, err := New(Options{CommandUse: "app"}, reloadConfig{})
	if err != nil {
		t.Fatal(err)
	}
	builder := func(vcflag.Metadata, PrototypeSchema, []byte) (ReloadDecoderFactory[int], error) {
		return func(RegistrationMetadata) (ReloadDecoder[int], error) { return nil, nil }, nil
	}
	runE, err := NewReloadRunE[int](manager, func() int { return 0 }, builder, ReloadOptions[int]{Clone: identity[int]})
	if err != nil {
		t.Fatal(err)
	}
	if err := runE(nil, nil); err == nil {
		t.Fatal("nil command accepted")
	}
	if err := runE(&cobra.Command{Use: "foreign"}, nil); !errors.Is(err, ErrForeignCommand) {
		t.Fatalf("foreign command error = %v", err)
	}
}
