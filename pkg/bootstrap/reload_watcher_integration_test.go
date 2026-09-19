package bootstrap

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/ductrung-nguyen/goapp-utils/pkg/watchapi"
)

func TestReloadManagerDefaultFactoryReloadsAndCloses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(path, []byte("1"), 0o600); err != nil {
		t.Fatal(err)
	}
	reg := lifecycleRegistration(1, func(RegistrationMetadata) (ReloadDecoder[int], error) {
		return func(r io.Reader, _ string, _ RegistrationMetadata) (int, error) {
			b, err := io.ReadAll(r)
			if err != nil {
				return 0, err
			}
			return strconv.Atoi(string(b))
		}, nil
	})
	reg.selected = path
	updates := make(chan int, 2)
	m, err := NewReloadManager(context.Background(), reg, ReloadOptions[int]{
		Clone: identity[int],
		Subscribers: []Subscriber[int]{func(v int, ev ReloadEvent[int]) {
			if ev.Kind == Updated {
				updates <- v
			}
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Start(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("2"), 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-updates:
		if got != 2 {
			t.Fatalf("updated value = %d, want 2", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for default filewatcher update")
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	if err := m.Wait(); err != nil {
		t.Fatalf("Wait after Close = %v", err)
	}
}

func TestReloadManagerFactoryErrorPropagatesToWait(t *testing.T) {
	reg := lifecycleRegistration(1, func(RegistrationMetadata) (ReloadDecoder[int], error) { return nil, nil })
	want := errors.New("watch factory failed")
	m, err := NewReloadManager(context.Background(), reg, ReloadOptions[int]{
		Clone: identity[int],
		Factory: func(context.Context, []watchapi.Target) (watchapi.Source, error) {
			return nil, want
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Start(); !errors.Is(err, want) {
		t.Fatalf("Start error = %v, want %v", err, want)
	}
	if err := m.Wait(); !errors.Is(err, want) {
		t.Fatalf("Wait error = %v, want %v", err, want)
	}
	if err := m.Close(); !errors.Is(err, want) {
		t.Fatalf("Close error = %v, want %v", err, want)
	}
}
