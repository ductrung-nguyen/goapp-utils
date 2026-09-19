package watchapi

import "context"

type Target struct{ ConfiguredPath, ParentPath string }
type Event struct {
	Op   uint32
	Path string
	Err  error
}
type Source interface {
	Add([]Target) error
	Events() <-chan Event
	Errors() <-chan error
	Close() error
}
type Factory func(context.Context, []Target) (Source, error)
