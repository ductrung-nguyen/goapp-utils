package bootstrap

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func TestBindEnvVarsToFlagsLocalBindsTypedLocalFlagsOnly(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	root.PersistentFlags().String("inherited-value", "", "")

	cmd := &cobra.Command{Use: "app"}
	cmd.PersistentFlags().String("persistent-value", "", "")
	cmd.Flags().String("local-value", "", "")
	cmd.Flags().Int("local-count", 0, "")
	root.AddCommand(cmd)

	v := viper.New()
	t.Setenv("APP_LOCAL_VALUE", "from-env")
	t.Setenv("APP_LOCAL_COUNT", "42")

	if err := BindEnvVarsToFlagsLocal(v, cmd, "APP", nil, []string{"local-value", "local-count"}); err != nil {
		t.Fatal(err)
	}
	if got := v.GetString("local-value"); got != "from-env" {
		t.Fatalf("local-value = %q, want from-env", got)
	}
	if got := v.GetInt("local-count"); got != 42 {
		t.Fatalf("local-count = %d, want 42", got)
	}
}

func TestBindEnvVarsToFlagsLocalRejectsInheritedAndPersistentFlags(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	root.PersistentFlags().String("inherited-value", "", "")
	cmd := &cobra.Command{Use: "app"}
	cmd.PersistentFlags().String("persistent-value", "", "")
	cmd.Flags().String("local-value", "", "")
	root.AddCommand(cmd)

	for _, name := range []string{"inherited-value", "persistent-value"} {
		t.Run(name, func(t *testing.T) {
			if err := BindEnvVarsToFlagsLocal(viper.New(), cmd, "APP", nil, []string{name}); err == nil {
				t.Fatalf("expected %q to be rejected as non-local", name)
			}
		})
	}
}

func TestBindEnvVarsToFlagsLocalRejectsUnknownFlag(t *testing.T) {
	cmd := &cobra.Command{Use: "app"}
	if err := BindEnvVarsToFlagsLocal(viper.New(), cmd, "APP", nil, []string{"missing"}); err == nil {
		t.Fatal("expected unknown flag error")
	}
}
