// Package bootstrap provides opt-in application configuration startup helpers.
package bootstrap

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"

	"github.com/ductrung-nguyen/goapp-utils/pkg/vcflag"
	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var (
	ErrForeignCommand = errors.New("bootstrap: command is not owned by manager")
	ErrInvalidOutput  = errors.New("bootstrap: output must be a non-nil pointer to the prototype type")
	ErrAlreadyLoaded  = errors.New("bootstrap: manager has already attempted loading")
	ErrInvalidPath    = errors.New("bootstrap: invalid configuration path")
)

// ConfigNotFoundError reports an absent configuration file.
type ConfigNotFoundError struct{ Path string }

func (e *ConfigNotFoundError) Error() string {
	return fmt.Sprintf("bootstrap: configuration file not found: %s", e.Path)
}
func (e *ConfigNotFoundError) Unwrap() error { return viper.ConfigFileNotFoundError{} }

// InvalidPathError reports a path which cannot be safely resolved inside a search root.
type InvalidPathError struct{ Path, Reason string }

func (e *InvalidPathError) Error() string {
	return fmt.Sprintf("%v %q: %s", ErrInvalidPath, e.Path, e.Reason)
}
func (e *InvalidPathError) Unwrap() error { return ErrInvalidPath }

// MissingPolicy controls whether an absent configuration file is an error.
type MissingPolicy int

const (
	MissingConfigAllowed MissingPolicy = iota
	MissingConfigRequired
)

// Options configures a bootstrap manager.
type Options struct {
	CommandUse        string
	ConfigFlagName    string
	DisableConfigFlag bool
	ExplicitFile      string
	ConfigName        string
	ConfigType        string
	SearchPaths       []string
	MissingPolicy     MissingPolicy
	EnableEnv         bool
	EnvPrefix         string
}

// InvalidPrototypeError reports a configuration prototype that is not exactly
// a struct value (T) or a non-nil pointer to a struct value (*T).
type InvalidPrototypeError struct {
	Type reflect.Type
}

func (e *InvalidPrototypeError) Error() string {
	if e.Type == nil {
		return "bootstrap: configuration prototype must be a struct or non-nil pointer to a struct"
	}
	return fmt.Sprintf("bootstrap: configuration prototype type %s must be a struct or non-nil pointer to a struct", e.Type)
}

// CommandConflictError reports an attempted child registration whose first
// command word is already owned by the parent.
type CommandConflictError struct {
	Parent string
	Name   string
}

func (e *CommandConflictError) Error() string {
	return fmt.Sprintf("bootstrap: parent command %q already has child command %q", e.Parent, e.Name)
}

// Manager owns the prototype type and the per-application bootstrap state.
type Manager struct {
	mu            sync.Mutex
	prototypeType reflect.Type
	options       Options
	viper         *viper.Viper
	flagNames     []string
	command       *cobra.Command
	loaded        bool
	prototype     PrototypeSchema
}

// New validates prototype and creates an independent bootstrap manager.
// The prototype's runtime value is never retained; only its canonical struct
// type is stored. Accepted forms are exactly T and *T, where T is a struct and
// *T is non-nil.
func New(options Options, prototype any) (*Manager, error) {
	prototypeType, err := canonicalPrototypeType(prototype)
	if err != nil {
		return nil, err
	}
	schema, err := newPrototypeSchema(prototypeType)
	if err != nil {
		return nil, fmt.Errorf("bootstrap: build prototype schema: %w", err)
	}
	options.SearchPaths = append([]string(nil), options.SearchPaths...)
	return &Manager{
		prototypeType: prototypeType,
		options:       options,
		prototype:     schema,
	}, nil
}

func canonicalPrototypeType(prototype any) (reflect.Type, error) {
	if prototype == nil {
		return nil, &InvalidPrototypeError{}
	}

	typeOfPrototype := reflect.TypeOf(prototype)
	valueOfPrototype := reflect.ValueOf(prototype)
	if typeOfPrototype.Kind() == reflect.Pointer {
		if valueOfPrototype.IsNil() || typeOfPrototype.Elem().Kind() != reflect.Struct {
			return nil, &InvalidPrototypeError{Type: typeOfPrototype}
		}
		return typeOfPrototype.Elem(), nil
	}
	if typeOfPrototype.Kind() != reflect.Struct {
		return nil, &InvalidPrototypeError{Type: typeOfPrototype}
	}
	return typeOfPrototype, nil
}

func (m *Manager) NewCommand() (*cobra.Command, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	options := m.options
	if options.ConfigName == "" {
		options.ConfigName = "config"
	}
	if options.ConfigFlagName == "" {
		options.ConfigFlagName = "config"
	}
	if options.CommandUse == "" {
		options.CommandUse = options.ConfigName
	}
	if err := validateCommandUse(options.CommandUse); err != nil {
		return nil, err
	}
	if options.EnableEnv && options.EnvPrefix == "" {
		return nil, fmt.Errorf("bootstrap: environment prefix is required")
	}
	if options.DisableConfigFlag && options.ExplicitFile == "" {
		return nil, fmt.Errorf("bootstrap: explicit file is required")
	}
	if !options.DisableConfigFlag && options.ExplicitFile != "" {
		return nil, fmt.Errorf("bootstrap: explicit file requires disabled config flag")
	}

	staged := &cobra.Command{Use: options.CommandUse}
	v := viper.New()
	prototype := reflect.New(m.prototypeType).Elem().Interface()
	if err := vcflag.GenerateFlags(prototype, v, staged); err != nil {
		return nil, fmt.Errorf("bootstrap: generate flags: %w", err)
	}
	if !options.DisableConfigFlag {
		if staged.Flags().Lookup(options.ConfigFlagName) != nil {
			return nil, fmt.Errorf("bootstrap: config flag %q collides with generated flag", options.ConfigFlagName)
		}
		staged.Flags().String(options.ConfigFlagName, "", "explicit configuration file")
	}
	flagNames := make([]string, 0)
	staged.Flags().VisitAll(func(flag *pflag.Flag) {
		if flag.Name != options.ConfigFlagName {
			flagNames = append(flagNames, flag.Name)
		}
	})
	if options.EnableEnv {
		if err := vcflag.BindEnvVarsToFlagsLocal(v, staged, options.EnvPrefix, nil, flagNames); err != nil {
			return nil, fmt.Errorf("bootstrap: bind environment: %w", err)
		}
	}
	m.options, m.viper, m.flagNames, m.command = options, v, flagNames, staged
	return staged, nil
}

func (m *Manager) NewSubcommand(parent *cobra.Command, use string, options Options, prototype any) (*Manager, *cobra.Command, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if parent == nil {
		return nil, nil, fmt.Errorf("bootstrap: parent command is nil")
	}
	if m.command == nil || parent != m.command {
		return nil, nil, ErrForeignCommand
	}
	if m.loaded {
		return nil, nil, ErrAlreadyLoaded
	}
	child, err := New(options, prototype)
	if err != nil {
		return nil, nil, err
	}
	child.options.CommandUse = use
	command, err := child.NewCommand()
	if err != nil {
		return nil, nil, err
	}
	name := command.Name()
	for _, existing := range parent.Commands() {
		if existing.Name() == name {
			return nil, nil, &CommandConflictError{Parent: parent.Name(), Name: name}
		}
	}
	parent.AddCommand(command)
	return child, command, nil
}

// Load resolves, reads, and decodes configuration exactly once.
func (m *Manager) Load(cmd *cobra.Command, out any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.loaded {
		return ErrAlreadyLoaded
	}
	if m.command == nil || cmd != m.command {
		return ErrForeignCommand
	}
	typeOfOutput := reflect.TypeOf(out)
	valueOfOutput := reflect.ValueOf(out)
	if typeOfOutput == nil || typeOfOutput.Kind() != reflect.Pointer || valueOfOutput.IsNil() || typeOfOutput.Elem() != m.prototypeType {
		return ErrInvalidOutput
	}
	m.loaded = true
	if err := m.loadConfigWithOptions(cmd, out, m.options); err != nil {
		return err
	}
	return nil
}

func (m *Manager) loadConfigWithOptions(cmd *cobra.Command, out any, options Options) error {
	o := options
	if o.ConfigType == "" {
		o.ConfigType = "yaml"
	}
	if o.ConfigType != "yaml" && o.ConfigType != "yml" {
		return fmt.Errorf("bootstrap: invalid config type %q", o.ConfigType)
	}
	if o.ConfigName == "" {
		o.ConfigName = "config"
	}
	if err := validateConfigName(o.ConfigName); err != nil {
		return err
	}
	roots, err := canonicalRoots(o.SearchPaths)
	if err != nil {
		return err
	}
	control := ""
	if !o.DisableConfigFlag {
		if f := cmd.LocalNonPersistentFlags().Lookup(o.ConfigFlagName); f != nil && f.Changed {
			control = f.Value.String()
			if control == "" {
				return &InvalidPathError{Path: control, Reason: "explicit selector is empty"}
			}
		}
	} else {
		control = o.ExplicitFile
	}
	if len(roots) == 0 && control == "" && o.MissingPolicy == MissingConfigRequired {
		return &ConfigNotFoundError{Path: o.ConfigName}
	}
	logger := logr.Discard()
	if control != "" {
		path, err := resolveExplicit(control, roots, o.ConfigType)
		if err != nil {
			return err
		}
		err = vcflag.InitConfigReader(m.viper, cmd, path, "", "", nil, o.EnvPrefix, &logger, false)
		if err != nil {
			return wrapReadError(path, err)
		}
	} else if len(roots) > 0 {
		if err := vcflag.InitConfigReader(m.viper, cmd, "", o.ConfigName, o.ConfigType, roots, o.EnvPrefix, &logger, false); err != nil {
			if _, ok := err.(viper.ConfigFileNotFoundError); ok {
				if o.MissingPolicy == MissingConfigRequired {
					return &ConfigNotFoundError{Path: o.ConfigName}
				}
			} else {
				return err
			}
		}
	}
	if err := m.viper.Unmarshal(out); err != nil {
		return fmt.Errorf("bootstrap: decode configuration: %w", err)
	}
	return nil
}

func wrapReadError(path string, err error) error {
	var notFound viper.ConfigFileNotFoundError
	if errors.As(err, &notFound) || errors.Is(err, os.ErrNotExist) {
		return &ConfigNotFoundError{Path: path}
	}
	return fmt.Errorf("bootstrap: read configuration %q: %w", path, err)
}

func canonicalRoots(paths []string) ([]string, error) {
	if len(paths) == 0 {
		paths = []string{"./configs", "."}
	}
	seen := map[string]bool{}
	result := make([]string, 0, len(paths))
	for _, raw := range paths {
		p, err := expandPath(raw)
		if err != nil {
			return nil, err
		}
		if p == "" {
			return nil, &InvalidPathError{Path: raw, Reason: "empty search root"}
		}
		abs, err := filepath.Abs(p)
		if err != nil {
			return nil, err
		}
		canon, err := filepath.EvalSymlinks(abs)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("bootstrap: search root %q: %w", raw, err)
		}
		canon, err = filepath.Abs(canon)
		if err != nil {
			return nil, err
		}
		if !seen[canon] {
			seen[canon] = true
			result = append(result, canon)
		}
	}
	return result, nil
}

func resolveExplicit(raw string, roots []string, typ string) (string, error) {
	p, err := expandPath(raw)
	if err != nil {
		return "", err
	}
	if p == "" {
		return "", &InvalidPathError{Path: raw, Reason: "empty path"}
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	if ext := strings.ToLower(filepath.Ext(abs)); ext != ".yaml" && ext != ".yml" {
		return "", &InvalidPathError{Path: raw, Reason: "configuration file must use .yaml or .yml"}
	}
	for _, root := range roots {
		var candidate string
		if real, e := filepath.EvalSymlinks(abs); e == nil {
			candidate = real
		} else if errors.Is(e, os.ErrNotExist) {
			parent, parentErr := filepath.EvalSymlinks(filepath.Dir(abs))
			if parentErr != nil {
				continue
			}
			candidate = filepath.Join(parent, filepath.Base(abs))
		} else {
			return "", fmt.Errorf("bootstrap: resolve configuration path %q: %w", raw, e)
		}
		candidate, err = filepath.Abs(candidate)
		if err != nil {
			return "", err
		}
		if !contained(root, candidate) {
			continue
		}
		if real, e := filepath.EvalSymlinks(abs); e == nil && !contained(root, real) {
			return "", &InvalidPathError{Path: raw, Reason: "symlink escapes search root"}
		}
		return candidate, nil
	}
	return "", &InvalidPathError{Path: raw, Reason: "path is outside configured search roots"}
}

func contained(root, path string) bool {
	r, err1 := filepath.Abs(root)
	p, err2 := filepath.Abs(path)
	if err1 != nil || err2 != nil {
		return false
	}
	rel, err := filepath.Rel(r, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func expandPath(raw string) (string, error) {
	if strings.HasPrefix(raw, "~") {
		home, ok := os.LookupEnv("HOME")
		if !ok || home == "" {
			return "", &InvalidPathError{Path: raw, Reason: "HOME is unset"}
		}
		if raw == "~" {
			raw = home
		} else if strings.HasPrefix(raw, "~/") {
			raw = filepath.Join(home, raw[2:])
		}
	}
	for i := 0; i < len(raw); i++ {
		if raw[i] != '$' {
			continue
		}
		start, end := i+1, i+1
		var name string
		if end < len(raw) && raw[end] == '{' {
			end++
			for end < len(raw) && raw[end] != '}' {
				end++
			}
			if end >= len(raw) {
				return "", &InvalidPathError{Path: raw, Reason: "unclosed environment reference"}
			}
			name = raw[start+1 : end]
			i = end
		} else {
			for end < len(raw) && (raw[end] == '_' || raw[end] >= 'A' && raw[end] <= 'Z' || raw[end] >= 'a' && raw[end] <= 'z' || raw[end] >= '0' && raw[end] <= '9') {
				end++
			}
			if end == start {
				return "", &InvalidPathError{Path: raw, Reason: "invalid environment reference"}
			}
			name = raw[start:end]
			i = end - 1
		}
		if !validEnvName(name) {
			return "", &InvalidPathError{Path: raw, Reason: "invalid environment reference"}
		}
		value, ok := os.LookupEnv(name)
		if !ok || value == "" {
			return "", &InvalidPathError{Path: raw, Reason: "environment variable is unset or empty"}
		}
	}
	return os.Expand(raw, func(key string) string { v, _ := os.LookupEnv(key); return v }), nil
}

func validateConfigName(name string) error {
	if name == "" || name == "." || name == ".." {
		return &InvalidPathError{Path: name, Reason: "configuration name must be a safe basename"}
	}
	if strings.ContainsAny(name, `/\\`) {
		return &InvalidPathError{Path: name, Reason: "configuration name must not contain path separators"}
	}
	if filepath.Base(name) != name {
		return &InvalidPathError{Path: name, Reason: "configuration name must be a safe basename"}
	}
	return nil
}

func validateCommandUse(use string) error {
	trimmed := strings.TrimSpace(use)
	if trimmed == "" || strings.ContainsAny(trimmed, "\r\n\t") || strings.HasPrefix(strings.Fields(trimmed)[0], "-") {
		return fmt.Errorf("bootstrap: invalid command use %q", use)
	}
	return nil
}

func validEnvName(s string) bool {
	if s == "" || (s[0] >= '0' && s[0] <= '9') {
		return false
	}
	for _, c := range s {
		if c != '_' && (c < 'A' || c > 'Z') && (c < 'a' || c > 'z') && (c < '0' || c > '9') {
			return false
		}
	}
	return true
}

// BindEnvVarsToFlagsLocal binds selected local flags to environment variables.
func BindEnvVarsToFlagsLocal(v *viper.Viper, cmd *cobra.Command, envPrefix string, logger *logr.Logger, flagNames []string) error {
	return vcflag.BindEnvVarsToFlagsLocal(v, cmd, envPrefix, logger, flagNames)
}
