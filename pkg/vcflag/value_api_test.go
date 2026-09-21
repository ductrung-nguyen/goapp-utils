package vcflag_test

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/ductrung-nguyen/goapp-utils/pkg/vcflag"
)

func TestMetadataUsesCanonicalValueType(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	if err := vcflag.GenerateFlags(struct {
		Port int `pflag:"port"`
	}{}, viper.New(), cmd); err != nil {
		t.Fatalf("generate flags: %v", err)
	}

	metadata, err := vcflag.CaptureMetadata(cmd)
	if err != nil {
		t.Fatalf("capture metadata: %v", err)
	}
	fields := metadata.Fields()
	if len(fields) != 1 {
		t.Fatalf("metadata fields = %d, want 1", len(fields))
	}

	// Keep the public metadata contract compile-checked: Field.Value and the
	// Fields accessor must expose the canonical vcflag.Value type.
	value := fields[0].Value
	if value.Kind != vcflag.ValueKindInt {
		t.Fatalf("value kind = %v, want %v", value.Kind, vcflag.ValueKindInt)
	}
}
