package bootstrap

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/spf13/cobra"
)

type prototypeConfig struct {
	Port int
}

func TestNewAcceptsValueAndPointerPrototypes(t *testing.T) {
	for _, prototype := range []any{prototypeConfig{}, &prototypeConfig{Port: 1234}} {
		manager, err := New(Options{}, prototype)
		if err != nil {
			t.Fatalf("New(%T) returned error: %v", prototype, err)
		}
		if manager.prototypeType != reflect.TypeOf(prototypeConfig{}) {
			t.Fatalf("prototype type = %v, want %v", manager.prototypeType, reflect.TypeOf(prototypeConfig{}))
		}
	}
}

func TestNewRejectsInvalidPrototypeForms(t *testing.T) {
	var nilConfig *prototypeConfig
	cases := []any{nil, nilConfig, 1, &[]string{}, &nilConfig}
	for _, prototype := range cases {
		_, err := New(Options{}, prototype)
		if err == nil {
			t.Fatalf("New(%T) returned nil error", prototype)
		}
		var validationErr *InvalidPrototypeError
		if !errors.As(err, &validationErr) {
			t.Fatalf("New(%T) error %T does not wrap InvalidPrototypeError: %v", prototype, err, err)
		}
	}
}

func TestNewCopiesPrototypeTypeOnly(t *testing.T) {
	prototype := prototypeConfig{Port: 1234}
	manager, err := New(Options{}, prototype)
	if err != nil {
		t.Fatal(err)
	}
	if manager.prototypeType != reflect.TypeOf(prototypeConfig{}) {
		t.Fatalf("prototype type = %v, want canonical value type", manager.prototypeType)
	}
}

func TestCommandUseDefaultsAndValidation(t *testing.T) {
	manager, err := New(Options{}, prototypeConfig{})
	if err != nil {
		t.Fatal(err)
	}
	cmd, err := manager.NewCommand()
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Use != "config" {
		t.Fatalf("Use = %q, want config", cmd.Use)
	}
	for _, use := range []string{"   ", "--bad", "app\nother"} {
		manager, _ := New(Options{CommandUse: use}, prototypeConfig{})
		if _, err := manager.NewCommand(); err == nil {
			t.Fatalf("NewCommand(%q) returned nil error", use)
		}
	}
}

func TestNewCommandPublishesGeneratedFlagsAndControlFlag(t *testing.T) {
	manager, err := New(Options{CommandUse: "app", EnableEnv: true, EnvPrefix: "APP"}, prototypeConfig{})
	if err != nil {
		t.Fatal(err)
	}
	cmd, err := manager.NewCommand()
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Flags().Lookup("Port") == nil || cmd.Flags().Lookup("config") == nil {
		t.Fatalf("generated/control flags missing")
	}
	if cmd.Flags().Lookup("config").Usage == "" {
		t.Fatal("control flag has no usage")
	}
}

type collisionConfig struct{ Config string }

func TestNewCommandRejectsControlFlagCollision(t *testing.T) {
	manager, err := New(Options{ConfigFlagName: "Config"}, collisionConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.NewCommand(); err == nil {
		t.Fatal("expected control flag collision")
	}
}

func TestNewSubcommandRejectsDuplicateFirstWordWithoutMutatingParent(t *testing.T) {
	manager, err := New(Options{CommandUse: "root"}, prototypeConfig{})
	if err != nil {
		t.Fatal(err)
	}
	parent, err := manager.NewCommand()
	if err != nil {
		t.Fatal(err)
	}
	parent.AddCommand(&cobra.Command{Use: "serve existing"})
	before := len(parent.Commands())

	_, _, err = manager.NewSubcommand(parent, "serve another", Options{}, prototypeConfig{})
	if err == nil {
		t.Fatal("expected duplicate child command conflict")
	}
	var conflictErr *CommandConflictError
	if !errors.As(err, &conflictErr) {
		t.Fatalf("error %T does not wrap CommandConflictError: %v", err, err)
	}
	if conflictErr.Name != "serve" || conflictErr.Parent != "root" {
		t.Fatalf("conflict = %+v, want name serve and parent root", conflictErr)
	}
	if got := len(parent.Commands()); got != before {
		t.Fatalf("parent command count = %d, want unchanged count %d", got, before)
	}
}

func TestNewSubcommandRequiresOwnedRegisteredParent(t *testing.T) {
	manager, err := New(Options{CommandUse: "root"}, prototypeConfig{})
	if err != nil {
		t.Fatal(err)
	}
	foreign := &cobra.Command{Use: "foreign"}
	if _, _, err := manager.NewSubcommand(foreign, "child", Options{}, prototypeConfig{}); !errors.Is(err, ErrForeignCommand) {
		t.Fatalf("foreign parent error = %v, want ErrForeignCommand", err)
	}
	parent, err := manager.NewCommand()
	if err != nil {
		t.Fatal(err)
	}
	childManager, child, err := manager.NewSubcommand(parent, "child", Options{}, prototypeConfig{})
	if err != nil || childManager == nil || child == nil {
		t.Fatalf("owned parent child creation: manager=%v child=%v err=%v", childManager, child, err)
	}
	if child.Parent() != parent {
		t.Fatal("child was not registered with parent")
	}
}

func TestLoadRejectsForeignAndInvalidCommandsOrOutputs(t *testing.T) {
	manager, err := New(Options{CommandUse: "app"}, prototypeConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Load(&cobra.Command{Use: "foreign"}, &prototypeConfig{}); !errors.Is(err, ErrForeignCommand) {
		t.Fatalf("foreign command error = %v, want ErrForeignCommand", err)
	}
	cmd, err := manager.NewCommand()
	if err != nil {
		t.Fatal(err)
	}
	for _, out := range []any{nil, prototypeConfig{}, (*prototypeConfig)(nil), new(int)} {
		if err := manager.Load(cmd, out); !errors.Is(err, ErrInvalidOutput) {
			t.Errorf("Load(%T) error = %v, want ErrInvalidOutput", out, err)
		}
	}
}
func TestLoadRejectsSecondInvocation(t *testing.T) {
	manager, err := New(Options{CommandUse: "app", MissingPolicy: MissingConfigRequired}, prototypeConfig{})
	if err != nil {
		t.Fatal(err)
	}
	cmd, err := manager.NewCommand()
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Load(cmd, new(prototypeConfig)); err == nil {
		t.Fatal("expected configuration loading stub error")
	}
	if err := manager.Load(cmd, new(prototypeConfig)); !errors.Is(err, ErrAlreadyLoaded) {
		t.Fatalf("second Load error = %v, want ErrAlreadyLoaded", err)
	}
}

func TestLoadExplicitMissingConfigReturnsTypedErrorWhenOptional(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "missing.yaml")

	manager, err := New(Options{
		CommandUse:        "app",
		DisableConfigFlag: true,
		ExplicitFile:      missing,
		SearchPaths:       []string{dir},
		MissingPolicy:     MissingConfigAllowed,
	}, prototypeConfig{})
	if err != nil {
		t.Fatal(err)
	}
	cmd, err := manager.NewCommand()
	if err != nil {
		t.Fatal(err)
	}

	err = manager.Load(cmd, new(prototypeConfig))
	var notFound *ConfigNotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("Load missing explicit config error = %v, want ConfigNotFoundError", err)
	}
	expectedDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	expectedPath := filepath.Join(expectedDir, filepath.Base(missing))
	if notFound.Path != expectedPath {
		t.Fatalf("ConfigNotFoundError path = %q, want %q", notFound.Path, expectedPath)
	}
}

func TestLoadSkipsMissingSearchRootAndUsesNamedRoot(t *testing.T) {
	root := t.TempDir()
	missingRoot := filepath.Join(root, "missing")
	configRoot := filepath.Join(root, "named")
	if err := os.Mkdir(configRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(configRoot, "config.yaml")
	if err := os.WriteFile(configPath, []byte("port: 4321\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	manager, err := New(Options{
		CommandUse:    "app",
		SearchPaths:   []string{missingRoot, configRoot},
		MissingPolicy: MissingConfigAllowed,
	}, prototypeConfig{})
	if err != nil {
		t.Fatal(err)
	}
	cmd, err := manager.NewCommand()
	if err != nil {
		t.Fatal(err)
	}

	config := new(prototypeConfig)
	if err := manager.Load(cmd, config); err != nil {
		t.Fatalf("Load with missing optional search root = %v", err)
	}
	if config.Port != 4321 {
		t.Fatalf("loaded port = %d, want 4321", config.Port)
	}
}

func TestLoadRejectsUnsafeConfigNameBeforeNamedSearch(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"../secret", `subdir/config`, `..\\secret`} {
		manager, err := New(Options{
			CommandUse:    "app",
			ConfigName:    name,
			SearchPaths:   []string{root},
			MissingPolicy: MissingConfigAllowed,
		}, prototypeConfig{})
		if err != nil {
			t.Fatal(err)
		}
		cmd, err := manager.NewCommand()
		if err != nil {
			t.Fatal(err)
		}
		err = manager.Load(cmd, new(prototypeConfig))
		var invalid *InvalidPathError
		if !errors.As(err, &invalid) {
			t.Fatalf("ConfigName %q error = %v, want InvalidPathError", name, err)
		}
	}
}

func TestLoadAllMissingSearchRootsDoesNotUseViperDefaults(t *testing.T) {
	root := t.TempDir()
	defaultConfig := filepath.Join(root, "config.yaml")
	if err := os.WriteFile(defaultConfig, []byte("port: 9999\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	missingRoot := filepath.Join(root, "missing")
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })

	manager, err := New(Options{
		CommandUse:    "app",
		SearchPaths:   []string{missingRoot},
		MissingPolicy: MissingConfigAllowed,
	}, prototypeConfig{})
	if err != nil {
		t.Fatal(err)
	}
	cmd, err := manager.NewCommand()
	if err != nil {
		t.Fatal(err)
	}
	config := new(prototypeConfig)
	if err := manager.Load(cmd, config); err != nil {
		t.Fatalf("Load with all missing roots = %v", err)
	}
	if config.Port != 0 {
		t.Fatalf("loaded port = %d, want 0 without configured roots", config.Port)
	}
}

func TestLoadOptionalMissingNamedConfigDecodesStagedValues(t *testing.T) {
	root := t.TempDir()
	missingRoot := filepath.Join(root, "missing")

	tests := []struct {
		name     string
		cliValue string
		envValue string
		want     int
	}{
		{name: "cli", cliValue: "1234", want: 1234},
		{name: "environment", envValue: "2345", want: 2345},
		{name: "cli wins environment", cliValue: "3456", envValue: "4567", want: 3456},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.envValue != "" {
				t.Setenv("APP_PORT", test.envValue)
			}
			manager, err := New(Options{
				CommandUse:    "app",
				ConfigName:    "named",
				SearchPaths:   []string{missingRoot},
				MissingPolicy: MissingConfigAllowed,
				EnableEnv:     true,
				EnvPrefix:     "APP",
			}, prototypeConfig{})
			if err != nil {
				t.Fatal(err)
			}
			cmd, err := manager.NewCommand()
			if err != nil {
				t.Fatal(err)
			}
			if test.cliValue != "" {
				if err := cmd.Flags().Set("Port", test.cliValue); err != nil {
					t.Fatal(err)
				}
			}

			config := new(prototypeConfig)
			if err := manager.Load(cmd, config); err != nil {
				t.Fatalf("Load optional missing named config = %v", err)
			}
			if config.Port != test.want {
				t.Fatalf("loaded port = %d, want %d", config.Port, test.want)
			}
		})
	}
}
