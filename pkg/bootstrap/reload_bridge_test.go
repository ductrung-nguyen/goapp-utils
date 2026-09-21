package bootstrap

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ductrung-nguyen/goapp-utils/pkg/vcflag"
	"github.com/spf13/cobra"
)

type reloadConfig struct {
	Port int `mapstructure:"port" pflag:"port"`
}

func TestPrepareRegistrationUsesCanonicalSelectedPath(t *testing.T) {
	root := t.TempDir()
	config := filepath.Join(root, "config.yaml")
	if err := os.WriteFile(config, []byte("port: 7\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	manager, err := New(Options{CommandUse: "app", SearchPaths: []string{root}}, reloadConfig{})
	if err != nil {
		t.Fatal(err)
	}
	cmd, err := manager.NewCommand()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Flags().Set("config", config); err != nil {
		t.Fatal(err)
	}
	md, err := vcflag.CaptureMetadata(cmd)
	if err != nil {
		t.Fatal(err)
	}
	regMD, err := CaptureRegistrationMetadata(manager, manager.Options(), md)
	if err != nil {
		t.Fatal(err)
	}
	factory, err := NewYAMLReloadDecoderFactory[reloadConfig](md, manager.PrototypeSchema(), manager.PrototypeDefaults())
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := PrepareReloadRegistration(manager, manager.Options(), cmd, func() reloadConfig { return reloadConfig{} }, regMD, factory)
	if err != nil {
		t.Fatal(err)
	}
	registration, err := BuildReloadRegistration(manager, resolved)
	if err != nil {
		t.Fatal(err)
	}
	path, ok := registration.SelectedPath()
	if !ok {
		t.Fatal("selected path missing")
	}
	want, err := filepath.EvalSymlinks(config)
	if err != nil {
		t.Fatal(err)
	}
	if path != want {
		t.Fatalf("selected path = %q, want canonical %q", path, want)
	}
}

func TestYAMLFactoryUsesInvocationRegistrationAndCLIOverridesEnvironment(t *testing.T) {
	cmd := &cobra.Command{Use: "app"}
	// registration data rather than the inputs captured at factory creation.
	first := vcflag.Metadata{}
	factory, err := NewYAMLReloadDecoderFactory[reloadConfig](first, PrototypeSchema{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	manager, err := New(Options{CommandUse: "app", EnableEnv: true, EnvPrefix: "APP"}, reloadConfig{})
	if err != nil {
		t.Fatal(err)
	}
	cmd, err = manager.NewCommand()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Flags().Set("port", "11"); err != nil {
		t.Fatal(err)
	}
	md, err := vcflag.CaptureMetadata(cmd)
	if err != nil {
		t.Fatal(err)
	}
	regMD, err := CaptureRegistrationMetadata(manager, manager.Options(), md)
	if err != nil {
		t.Fatal(err)
	}
	decoder, err := factory(regMD)
	if err != nil {
		t.Fatal(err)
	}
	_ = os.Setenv("APP_PORT", "22")
	defer func() {
		if err := os.Unsetenv("APP_PORT"); err != nil {
			t.Errorf("unset APP_PORT: %v", err)
		}
	}()
	got, err := decoder(bytes.NewBufferString("port: 33\n"), "", regMD)
	if err != nil {
		t.Fatal(err)
	}
	if got.Port != 11 {
		t.Fatalf("port = %d, want CLI value 11", got.Port)
	}
}

func TestPrepareRegistrationAcceptsRuntimeEnvironmentBindings(t *testing.T) {
	manager, err := New(Options{
		CommandUse: "app",
		EnableEnv:  true,
		EnvPrefix:  "APP",
	}, reloadConfig{})
	if err != nil {
		t.Fatal(err)
	}
	cmd, err := manager.NewCommand()
	if err != nil {
		t.Fatal(err)
	}
	md, err := vcflag.CaptureMetadata(cmd)
	if err != nil {
		t.Fatal(err)
	}
	fields := md.Fields()
	var portField *vcflag.Field
	for i := range fields {
		if fields[i].Name == "port" {
			portField = &fields[i]
			break
		}
	}
	if portField == nil || portField.Env.Name != "APP_PORT" {
		t.Fatalf("captured environment binding = %#v, want APP_PORT", fields)
	}
	registrationMetadata, err := CaptureRegistrationMetadata(manager, manager.Options(), md)
	if err != nil {
		t.Fatalf("capture registration metadata: %v", err)
	}
	factory, err := NewYAMLReloadDecoderFactory[reloadConfig](md, manager.PrototypeSchema(), manager.PrototypeDefaults())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PrepareReloadRegistration(manager, manager.Options(), cmd, func() reloadConfig { return reloadConfig{} }, registrationMetadata, factory); err != nil {
		t.Fatalf("prepare registration: %v", err)
	}
}

func TestCaptureRegistrationMetadataRejectsStructuralMismatchWithEnvironment(t *testing.T) {
	manager, err := New(Options{CommandUse: "app", EnableEnv: true, EnvPrefix: "APP"}, reloadConfig{})
	if err != nil {
		t.Fatal(err)
	}
	cmd, err := manager.NewCommand()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Flags().Lookup("port").Usage = "different usage"
	md, err := vcflag.CaptureMetadata(cmd)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CaptureRegistrationMetadata(manager, manager.Options(), md); err == nil {
		t.Fatal("expected structural metadata mismatch rejection")
	}
}

func TestYAMLFactoryRejectsInvocationMetadataSchemaMismatch(t *testing.T) {
	manager, err := New(Options{CommandUse: "app"}, reloadConfig{})
	if err != nil {
		t.Fatal(err)
	}
	cmd, err := manager.NewCommand()
	if err != nil {
		t.Fatal(err)
	}
	md, err := vcflag.CaptureMetadata(cmd)
	if err != nil {
		t.Fatal(err)
	}
	registrationMetadata, err := CaptureRegistrationMetadata(manager, manager.Options(), md)
	if err != nil {
		t.Fatal(err)
	}
	factory, err := NewYAMLReloadDecoderFactory[reloadConfig](md, manager.PrototypeSchema(), manager.PrototypeDefaults())
	if err != nil {
		t.Fatal(err)
	}
	decoder, err := factory(registrationMetadata)
	if err != nil {
		t.Fatal(err)
	}
	registrationMetadata.fields[0].Name = "different"
	got, err := decoder(bytes.NewBufferString("port: 33\n"), "", registrationMetadata)
	if !errors.Is(err, ErrInvalidRegistration) {
		t.Fatalf("decode error = %v, want ErrInvalidRegistration", err)
	}
	if got != (reloadConfig{}) {
		t.Fatalf("decoded output = %#v, want zero value", got)
	}
}

func TestPrepareRegistrationRejectsForeignMetadataAndOptions(t *testing.T) {
	m1, err := New(Options{CommandUse: "one"}, reloadConfig{})
	if err != nil {
		t.Fatal(err)
	}
	c1, err := m1.NewCommand()
	if err != nil {
		t.Fatal(err)
	}
	md, err := vcflag.CaptureMetadata(c1)
	if err != nil {
		t.Fatal(err)
	}
	registrationMetadata, err := CaptureRegistrationMetadata(m1, m1.Options(), md)
	if err != nil {
		t.Fatal(err)
	}
	m2, err := New(Options{CommandUse: "two"}, reloadConfig{})
	if err != nil {
		t.Fatal(err)
	}
	c2, err := m2.NewCommand()
	if err != nil {
		t.Fatal(err)
	}
	factory, err := NewYAMLReloadDecoderFactory[reloadConfig](md, m1.PrototypeSchema(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PrepareReloadRegistration(m2, m2.Options(), c2, func() reloadConfig { return reloadConfig{} }, registrationMetadata, factory); err == nil {
		t.Fatal("expected foreign metadata rejection")
	}
}

func TestOptionsDefensivelyCopiesSearchPaths(t *testing.T) {
	paths := []string{"/one", "/two"}
	manager, err := New(Options{CommandUse: "app", SearchPaths: paths}, reloadConfig{})
	if err != nil {
		t.Fatal(err)
	}
	paths[0] = "/mutated"
	got := manager.Options()
	got.SearchPaths[1] = "/changed"
	gotAgain := manager.Options()
	if gotAgain.SearchPaths[0] != "/one" || gotAgain.SearchPaths[1] != "/two" {
		t.Fatalf("manager SearchPaths mutated through caller/accessor: %#v", gotAgain.SearchPaths)
	}
}
