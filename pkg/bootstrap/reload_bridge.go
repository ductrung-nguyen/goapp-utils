package bootstrap

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sync"

	"github.com/ductrung-nguyen/goapp-utils/pkg/vcflag"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

func reloadCandidates(options Options, roots []string, selected string) ([]string, error) {
	if selected != "" || len(roots) == 0 {
		return nil, nil
	}
	name := options.ConfigName
	if name == "" {
		name = "config"
	}
	typ := options.ConfigType
	if typ == "" {
		typ = "yaml"
	}
	candidates := make([]string, 0, len(roots))
	for _, root := range roots {
		candidates = append(candidates, filepath.Join(root, name+"."+typ))
	}
	return candidates, nil
}

type PrototypeSchema struct {
	fields   []vcflag.Field
	defaults []vcflag.KeyValue
}

func newPrototypeSchema(t reflect.Type) (PrototypeSchema, error) {
	cmd := &cobra.Command{Use: "prototype"}
	v := viper.New()
	value := reflect.New(t).Elem().Interface()
	if err := vcflag.GenerateFlags(value, v, cmd); err != nil {
		return PrototypeSchema{}, fmt.Errorf("generate flags: %w", err)
	}
	md, err := vcflag.CaptureMetadata(cmd)
	if err != nil {
		return PrototypeSchema{}, fmt.Errorf("capture metadata: %w", err)
	}
	fields := md.Fields()
	defs := make([]vcflag.KeyValue, 0, len(fields))
	for _, f := range fields {
		defs = append(defs, vcflag.KeyValue{Key: f.Name, Value: f.Value.DefValue})
	}
	return PrototypeSchema{fields: fields, defaults: defs}, nil
}
func (s PrototypeSchema) Fields() []vcflag.Field { return append([]vcflag.Field(nil), s.fields...) }
func (s PrototypeSchema) Defaults() []vcflag.KeyValue {
	return append([]vcflag.KeyValue(nil), s.defaults...)
}
func (m *Manager) PrototypeSchema() PrototypeSchema {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.prototype
}
func (m *Manager) PrototypeDefaults() []byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, _ := yaml.Marshal(reflect.New(m.prototypeType).Elem().Interface())
	return b
}
func (m *Manager) Options() Options {
	m.mu.Lock()
	defer m.mu.Unlock()
	o := m.options
	o.SearchPaths = append([]string(nil), o.SearchPaths...)
	return o
}
func (m *Manager) CaptureMetadata(cmd *cobra.Command) (*cobra.Command, vcflag.Metadata, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cmd == nil || cmd != m.command {
		return nil, vcflag.Metadata{}, ErrForeignCommand
	}
	md, err := vcflag.CaptureMetadata(cmd)
	return cmd, md, err
}

type RegistrationMetadata struct {
	owner    *Manager
	fields   []vcflag.Field
	schema   PrototypeSchema
	defaults []byte
	options  Options
}

func CaptureRegistrationMetadata(m *Manager, options Options, raw vcflag.Metadata) (RegistrationMetadata, error) {
	if m == nil {
		return RegistrationMetadata{}, errors.New("bootstrap: nil manager")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if !reflect.DeepEqual(options, m.options) {
		return RegistrationMetadata{}, ErrInvalidRegistration
	}
	fields := raw.Fields()
	filtered := make([]vcflag.Field, 0, len(fields))
	for _, field := range fields {
		if field.Name == m.options.ConfigFlagName || field.Name == "help" {
			continue
		}
		filtered = append(filtered, field)
	}
	if !metadataMatchesStructure(filtered, m.prototype.fields) {
		return RegistrationMetadata{}, ErrInvalidRegistration
	}
	fields = filtered
	return RegistrationMetadata{
		owner:  m,
		fields: fields,
		schema: m.prototype,
		defaults: func() []byte {
			b, _ := yaml.Marshal(reflect.New(m.prototypeType).Elem().Interface())
			return b
		}(),
		options: options,
	}, nil
}

// metadataMatchesStructure validates fields generated from the prototype.
// Environment bindings are attached during command setup and are runtime metadata,
// so they are intentionally excluded from this structural comparison.
func metadataMatchesStructure(got, want []vcflag.Field) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i].Name != want[i].Name || got[i].Usage != want[i].Usage {
			return false
		}
		if got[i].Value.Kind != want[i].Value.Kind || got[i].Value.Type != want[i].Value.Type || got[i].Value.DefValue != want[i].Value.DefValue {
			return false
		}
	}
	return true
}

type ReloadDecoder[T any] func(io.Reader, string, RegistrationMetadata) (T, error)
type ReloadDecoderFactory[T any] func(RegistrationMetadata) (ReloadDecoder[T], error)

func NewYAMLReloadDecoderFactory[T any](raw vcflag.Metadata, schema PrototypeSchema, prototypeDefaults []byte) (ReloadDecoderFactory[T], error) {
	capturedFields := raw.Fields()
	if len(schema.fields) > 0 {
		capturedFields = append([]vcflag.Field(nil), schema.fields...)
	}
	capturedDefaults := append([]byte(nil), prototypeDefaults...)
	return func(reg RegistrationMetadata) (ReloadDecoder[T], error) {
		fields, defaults := reg.fields, reg.defaults
		return func(r io.Reader, _ string, supplied RegistrationMetadata) (T, error) {
			var out T
			if supplied.owner != reg.owner || !reflect.DeepEqual(supplied.options, reg.options) || (len(capturedFields) > 0 && !metadataMatchesStructure(supplied.fields, capturedFields)) {
				return out, ErrInvalidRegistration
			}
			v := viper.New()
			v.SetConfigType("yaml")
			for _, f := range fields {
				v.SetDefault(f.Name, f.Value.DefValue)
			}
			if len(defaults) > 0 {
				if err := v.ReadConfig(bytes.NewReader(defaults)); err != nil {
					return out, err
				}
			}
			if len(capturedDefaults) > 0 && len(defaults) == 0 {
				if err := v.ReadConfig(bytes.NewReader(capturedDefaults)); err != nil {
					return out, err
				}
			}
			if err := v.ReadConfig(r); err != nil {
				return out, err
			}
			for _, f := range supplied.fields {
				if f.Env.Name != "" {
					if x, ok := os.LookupEnv(f.Env.Name); ok {
						v.Set(f.Name, x)
					}
				}
			}
			for _, f := range supplied.fields {
				if f.Value.Changed {
					v.Set(f.Name, f.Value.String)
				}
			}
			if err := v.Unmarshal(&out); err != nil {
				return out, err
			}
			return out, nil
		}, nil
	}, nil
}

type ResolvedReloadSources[T any] struct {
	mu           sync.Mutex
	owner        *Manager
	initial      T
	missing      bool
	missingValue T
	selected     string
	candidates   []string
	metadata     RegistrationMetadata
	policy       MissingPolicy
	zero         func() T
	factory      ReloadDecoderFactory[T]
	built        bool
}

func (r *ResolvedReloadSources[T]) InitialSnapshot() (T, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.missing {
		return r.missingValue, false
	}
	return r.initial, true
}
func (r *ResolvedReloadSources[T]) InitialMissing() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.missing
}
func (r *ResolvedReloadSources[T]) Zero() (T, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.zero == nil {
		var z T
		return z, false
	}
	return r.zero(), true
}
func PrepareReloadRegistration[T any](m *Manager, options Options, cmd *cobra.Command, zero func() T, metadata RegistrationMetadata, factory ReloadDecoderFactory[T]) (*ResolvedReloadSources[T], error) {
	if m == nil || cmd == nil || zero == nil || factory == nil || metadata.owner != m {
		return nil, ErrInvalidRegistration
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.command != cmd {
		return nil, ErrForeignCommand
	}
	filteredFields := make([]vcflag.Field, 0, len(metadata.fields))
	for _, field := range metadata.fields {
		if field.Name == m.options.ConfigFlagName || field.Name == "help" {
			continue
		}
		filteredFields = append(filteredFields, field)
	}
	if !reflect.DeepEqual(options, m.options) || !reflect.DeepEqual(metadata.options, options) || !metadataMatchesStructure(filteredFields, m.prototype.fields) {
		return nil, ErrInvalidRegistration
	}
	if m.loaded {
		return nil, ErrAlreadyLoaded
	}
	m.loaded = true
	selected := selectPath(options, cmd)
	roots, err := canonicalRoots(options.SearchPaths)
	if err != nil {
		return nil, err
	}
	candidates, err := reloadCandidates(options, roots, selected)
	if err != nil {
		return nil, err
	}
	if selected != "" {
		selected, err = resolveExplicit(selected, roots, options.ConfigType)
		if err != nil {
			return nil, err
		}
		candidates = []string{selected}
	}
	var initial T
	if err := m.loadConfigWithOptions(cmd, &initial, options); err != nil {
		if _, ok := err.(*ConfigNotFoundError); ok && options.MissingPolicy == MissingConfigAllowed {
			return &ResolvedReloadSources[T]{owner: m, missing: true, missingValue: zero(), selected: selected, candidates: candidates, metadata: metadata, policy: options.MissingPolicy, zero: zero, factory: factory}, nil
		}
		return nil, err
	}
	if selected == "" && len(roots) == 0 && options.MissingPolicy == MissingConfigAllowed {
		return &ResolvedReloadSources[T]{owner: m, missing: true, missingValue: zero(), candidates: candidates, metadata: metadata, policy: options.MissingPolicy, zero: zero, factory: factory}, nil
	}
	if selected == "" && m.viper != nil {
		selected = m.viper.ConfigFileUsed()
	}
	return &ResolvedReloadSources[T]{owner: m, initial: initial, selected: selected, candidates: candidates, metadata: metadata, policy: options.MissingPolicy, zero: zero, factory: factory}, nil
}

type ReloadRegistration[T any] struct {
	initial      T
	missing      bool
	missingValue T
	selected     string
	candidates   []string
	metadata     RegistrationMetadata
	policy       MissingPolicy
	zero         func() T
	factory      ReloadDecoderFactory[T]
}

func (r *ReloadRegistration[T]) SelectedPath() (string, bool) { return r.selected, r.selected != "" }
func (r *ReloadRegistration[T]) Snapshot() (T, bool) {
	if r.missing {
		return cloneSnapshot(r.missingValue), false
	}
	return cloneSnapshot(r.initial), true
}

func cloneSnapshot[T any](v T) T {
	rv := reflect.ValueOf(v)
	return reflectClone(rv).Interface().(T)
}
func reflectClone(v reflect.Value) reflect.Value {
	if !v.IsValid() {
		return v
	}
	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			return reflect.Zero(v.Type())
		}
		p := reflect.New(v.Elem().Type())
		p.Elem().Set(reflectClone(v.Elem()))
		return p
	case reflect.Interface:
		if v.IsNil() {
			return reflect.Zero(v.Type())
		}
		x := reflectClone(v.Elem())
		out := reflect.New(v.Type()).Elem()
		out.Set(x)
		return out
	case reflect.Slice:
		if v.IsNil() {
			return reflect.Zero(v.Type())
		}
		out := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
		for i := 0; i < v.Len(); i++ {
			out.Index(i).Set(reflectClone(v.Index(i)))
		}
		return out
	case reflect.Map:
		if v.IsNil() {
			return reflect.Zero(v.Type())
		}
		out := reflect.MakeMapWithSize(v.Type(), v.Len())
		for _, k := range v.MapKeys() {
			out.SetMapIndex(reflectClone(k), reflectClone(v.MapIndex(k)))
		}
		return out
	case reflect.Struct:
		out := reflect.New(v.Type()).Elem()
		out.Set(v)
		for i := 0; i < v.NumField(); i++ {
			if out.Field(i).CanSet() {
				out.Field(i).Set(reflectClone(v.Field(i)))
			}
		}
		return out
	}
	return v
}

var ErrAlreadyBuilt = errors.New("bootstrap: reload registration already built")
var ErrInvalidRegistration = errors.New("invalid reload registration")

func selectPath(o Options, cmd *cobra.Command) string {
	if o.DisableConfigFlag {
		return o.ExplicitFile
	}
	if f := cmd.LocalNonPersistentFlags().Lookup(o.ConfigFlagName); f != nil && f.Changed {
		return f.Value.String()
	}
	return ""
}
func BuildReloadRegistration[T any](m *Manager, r *ResolvedReloadSources[T]) (*ReloadRegistration[T], error) {
	if m == nil || r == nil {
		return nil, ErrInvalidRegistration
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.owner != m {
		return nil, ErrInvalidRegistration
	}
	if r.built {
		return nil, ErrAlreadyBuilt
	}
	r.built = true
	return &ReloadRegistration[T]{initial: r.initial, missing: r.missing, missingValue: r.missingValue, selected: r.selected, candidates: append([]string(nil), r.candidates...), metadata: r.metadata, policy: r.policy, zero: r.zero, factory: r.factory}, nil
}

// ReloadDecoderFactoryBuilder builds a decoder factory from metadata captured
// for one parsed Cobra invocation.
type ReloadDecoderFactoryBuilder[T any] func(
	raw vcflag.Metadata,
	schema PrototypeSchema,
	prototypeDefaults []byte,
) (ReloadDecoderFactory[T], error)

// NewReloadRunE returns a Cobra-compatible handler for the reload lifecycle.
// It does not add commands, execute Cobra, install signal handlers, or own the
// process context.
func NewReloadRunE[T any](
	m *Manager,
	zero func() T,
	buildFactory ReloadDecoderFactoryBuilder[T],
	opts ReloadOptions[T],
) (func(*cobra.Command, []string) error, error) {
	if m == nil {
		return nil, errors.New("bootstrap: nil manager")
	}
	if zero == nil {
		return nil, errors.New("bootstrap: nil zero function")
	}
	if buildFactory == nil {
		return nil, errors.New("bootstrap: nil decoder factory builder")
	}

	return func(cmd *cobra.Command, _ []string) (err error) {
		if cmd == nil {
			return errors.New("bootstrap: nil command")
		}
		owned, raw, err := m.CaptureMetadata(cmd)
		if err != nil {
			return err
		}
		metadata, err := CaptureRegistrationMetadata(m, m.Options(), raw)
		if err != nil {
			return err
		}
		factory, err := buildFactory(raw, m.PrototypeSchema(), m.PrototypeDefaults())
		if err != nil {
			return err
		}
		resolved, err := PrepareReloadRegistration(m, m.Options(), owned, zero, metadata, factory)
		if err != nil {
			return err
		}
		registration, err := BuildReloadRegistration(m, resolved)
		if err != nil {
			return err
		}
		reloader, err := NewReloadManager(cmd.Context(), registration, opts)
		if err != nil {
			return err
		}
		defer func() {
			if closeErr := reloader.Close(cmd.Context()); err == nil && closeErr != nil {
				err = closeErr
			}
		}()
		if err = reloader.Start(); err != nil {
			return err
		}
		return reloader.Wait()
	}, nil
}
