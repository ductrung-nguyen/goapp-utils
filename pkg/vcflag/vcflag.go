package vcflag

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

//
// The functions in this file helps us to generate CLI flags from a (configuration) struct
// that is useful when our application wants to read the configuration from different channels:
// - configuration file
// - command line arguments
// - environment variables
// for example, if we have configuration struct:
// type Config struct {
//    nestedConfig struct {
//        a int
//        b string
//    }
//
//    c string
// }
//
// function GenerateFlags helps us to generate 3 arguments:
//     --config.nestedConfig.a
//     --config.nestedConfig.b
//
//

const (
	defaultConfigFile = "config"
)

var (
	durationKind = reflect.TypeOf(1 * time.Second).Kind()
)

// Init the config reader via Viper and Cobra objects.
// we can specify either the config file name, or the config name, and its type
// as well as the locations to find that config files
// if autoBindEnvVarsToFlags is true, the environment variables are bind automatically to the pflags
// name of the env vars is based on the struct
func InitConfigReader(
	viperObj *viper.Viper, cmd *cobra.Command,
	cfgFile string, cfgName string, cfgType string, configLocations []string,
	envPrefix string,
	logger *logr.Logger,
	autoBindEnvVarsToFlags bool,
) error {
	if cfgFile == "" && cfgName == "" {
		cfgFile = defaultConfigFile
	}
	// use config file
	// the type of the config will be inducted from the extension
	if cfgFile != "" {
		viperObj.SetConfigFile(cfgFile) // Register config file name with extension
	} else {
		viperObj.SetConfigName(cfgName) // Register config file name (no extension)
		viperObj.SetConfigType(cfgType) // Look for specific type
	}

	if len(configLocations) == 0 {
		configLocations = []string{
			"./configs", ".",
		}
	}

	for _, loc := range configLocations {
		logger.V(2).Info("Looking for config file", "folder", loc)
		viperObj.AddConfigPath(loc)
	}

	if err := viperObj.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found; ignore error if desired
			logger.Error(err, "Cannot find the config file")
		} else {
			// Config file was found but another error was produced
			logger.Error(err, "Cannot read the config file")
		}
		return err
	}

	if autoBindEnvVarsToFlags {
		BindEnvVarsToFlags(viperObj, cmd, envPrefix, logger)
	}

	return nil
}

// Init the config reader via Viper and Cobra objects.
// we can specify either the config file name, or the config name, and its type
// as well as the locations to find that config files
func BindEnvVarsToFlags(
	viperObj *viper.Viper, cmd *cobra.Command,
	envPrefix string,
	logger *logr.Logger,
) {
	// When we bind flags to environment variables expect that the
	// environment variables are prefixed, e.g. a flag like --number
	// binds to an environment variable CTOOLS_NUMBER. This helps
	// avoid conflicts.
	viperObj.SetEnvPrefix(envPrefix)
	// Bind to environment variables
	// Works great for simple config names, but needs help for names
	// like --favorite-color which we fix in the bindFlags function
	viperObj.AutomaticEnv()

	viperObj.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Bind the current command's flags to viper
	bindEnvVarsToFlags(cmd, viperObj, envPrefix, logger)
}

// getStructTag returns the value of a specific tag in the object structure
// for example, returns value of tag `json`, or tag `yaml`,....
func getStructTag(f reflect.StructField, tagName string) string {
	return strings.Trim(string(f.Tag.Get(tagName)), " \t")
}

// GenerateFlags creates flags based on the attributes of object `value`
// and bind them to viper
// this function should be called when initialize cobra command
// return error in case these is any issue when generating flags for CLI
func GenerateFlags(value interface{}, viperObj *viper.Viper, command *cobra.Command) error {
	original := reflect.ValueOf(value)
	copy := reflect.New(original.Type()).Elem()
	return generateFlags("", "", original, copy, "", viperObj, command)
}

// GenerateFlags creates flags based on the attributes of object `value`
// and bind them to viper
// this function should be called when initialize cobra command
// return error in case these is any issue when generating flags for CLI
func generateFlags(currentPath string, key string, value reflect.Value, copy reflect.Value, usage string,
	viperObj *viper.Viper, command *cobra.Command) error {

	path := key
	if currentPath != "" {
		path = fmt.Sprintf("%s.%s", currentPath, key)
	}

	typeOfT := value.Type()
	switch value.Kind() {
	case reflect.Pointer:
		elemType := value.Type().Elem()
		var elemValue reflect.Value
		if value.IsNil() {
			elemValue = reflect.New(elemType).Elem()
		} else {
			elemValue = value.Elem()
		}
		elemCopy := reflect.New(elemType).Elem()
		return generateFlags(currentPath, key, elemValue, elemCopy, usage, viperObj, command)
	case reflect.Struct:
		for idx := 0; idx < typeOfT.NumField(); idx += 1 {
			tag := getStructTag(typeOfT.Field(idx), "pflag")

			// if there is no value of tag pflag, try to use value of tag `mapstructure`
			if tag == "" {
				tag = getStructTag(typeOfT.Field(idx), "mapstructure")
			}

			usage := strings.Trim(getStructTag(typeOfT.Field(idx), "usage"), " \t")

			// if tag is "-", the user wants to skip this field
			if tag == "-" {
				continue
			} else if tag == "" {
				// if tag is empty, try to use field name to generate flag
				tag = strings.ReplaceAll(typeOfT.Field(idx).Name, "-", "_")
				tag = strings.ReplaceAll(tag, ".", "__")
				tag = strings.ReplaceAll(tag, " ", "_")
			}

			// GenerateFlags(path, tag, value.Field(idx), copy.Field(idx), viperObj, command)
			if err := generateFlags(path, tag, value.Field(idx), reflect.New(value.Field(idx).Type()).Elem(), usage, viperObj, command); err != nil {
				return err
			}
		}
		return nil
	case reflect.Bool:
		command.Flags().Bool(path, false, usage)
	case durationKind:
		command.Flags().Duration(path, 0*time.Second, usage)
	case reflect.Int:
		command.Flags().Int(path, 0, usage)
	case reflect.Uint:
		command.Flags().Uint(path, 0, usage)
	case reflect.Int16:
		command.Flags().Int16(path, 0, usage)
	case reflect.Uint16:
		command.Flags().Uint16(path, 0, usage)
	case reflect.Int32:
		command.Flags().Int32(path, 0, usage)
	case reflect.Uint32:
		command.Flags().Uint32(path, 0, usage)
	case reflect.Int64:
		command.Flags().Int64(path, 0, usage)
	case reflect.Uint64:
		command.Flags().Uint64(path, 0, usage)
	case reflect.Array:
	case reflect.Slice:
		copy.Set(reflect.MakeSlice(value.Type(), 1, 1))
		switch copy.Index(0).Kind() {
		case reflect.Int:
			command.Flags().IntSlice(path, []int{}, usage)
		case reflect.Uint:
			command.Flags().UintSlice(path, []uint{}, usage)
		case reflect.Int16:
			command.Flags().Int32Slice(path, []int32{}, usage)
		case reflect.Uint16:
			command.Flags().Int32Slice(path, []int32{}, usage)
		case reflect.Int32:
			command.Flags().Int32Slice(path, []int32{}, usage)
		case reflect.Uint32:
			command.Flags().Int32Slice(path, []int32{}, usage)
		case reflect.Int64:
			command.Flags().Int64Slice(path, []int64{}, usage)
		case reflect.Uint64:
			command.Flags().Int64Slice(path, []int64{}, usage)
		case reflect.String:
			command.Flags().StringSlice(path, []string{}, usage)
		default:
			return nil
		}
	case reflect.String:
		command.Flags().String(path, "", usage)
	default:
		command.Flags().String(path, "", usage)
	}
	return viperObj.BindPFlag(path, command.Flags().Lookup(path))
}

// BindEnvVarsToFlagsLocal binds only the named local flags to environment
// variables. It is additive to BindEnvVarsToFlags and does not inspect or
// mutate inherited persistent flags.
func BindEnvVarsToFlagsLocal(
	viperObj *viper.Viper,
	cmd *cobra.Command,
	envPrefix string,
	logger *logr.Logger,
	flagNames []string,
) error {
	viperObj.SetEnvPrefix(envPrefix)
	viperObj.AutomaticEnv()
	viperObj.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	for _, name := range flagNames {
		flag := cmd.LocalNonPersistentFlags().Lookup(name)
		if flag == nil {
			return fmt.Errorf("flag %q is not a local non-persistent flag", name)
		}
		envName := envNameForFlag(envPrefix, name)
		if logger != nil {
			logger.V(2).Info("Binding env to flag", "env", envName, "flag", name)
		}
		if err := viperObj.BindEnv(name, envName); err != nil {
			return fmt.Errorf("bind environment variable %q to flag %q: %w", envName, name, err)
		}
		if flag.Annotations == nil {
			flag.Annotations = map[string][]string{}
		}
		flag.Annotations["vcflag-env"] = []string{envName}
	}
	return nil
}

func envNameForFlag(envPrefix, flagName string) string {
	envVarSuffix := strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(flagName, "-", "_"), ".", "__"))
	return fmt.Sprintf("%s_%s", envPrefix, envVarSuffix)
}

// Bind each cobra flag to its associated viper configuration (config file and environment variable)
func bindEnvVarsToFlags(cmd *cobra.Command, v *viper.Viper, envPrefix string, logger *logr.Logger) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		// Environment variables can't have dashes in them, so bind them to their equivalent
		// keys with underscores, e.g. --favorite-color to MYAPP_FAVORITE_COLOR
		var envVarSuffix string
		if strings.Contains(f.Name, "-") || strings.Contains(f.Name, ".") {
			envVarSuffix = strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(f.Name, "-", "_"), ".", "__"))
		} else {
			envVarSuffix = strings.ToUpper(f.Name)
		}

		envName := fmt.Sprintf("%s_%s", envPrefix, envVarSuffix)
		logger.V(2).Info("Binding env to flag", "env", envName, "flag", f.Name)
		_ = v.BindEnv(f.Name, envName)
		if f.Usage != "" {
			f.Usage += ". "
		}
		f.Usage += "Overrided by Env Var " + envName

		// Apply the viper config value to the flag when the flag is not set and viper has a value
		// if !f.Changed && v.IsSet(f.Name) {
		// 	flagVal := v.Get(f.Name)
		// 	if reflect.TypeOf(flagVal).Kind() == reflect.Slice || reflect.TypeOf(flagVal).Kind() == reflect.Array {
		// 		slice := reflect.ValueOf(flagVal)
		// 		dataInStr := make([]string, slice.Len())

		// 		for i := 0; i < slice.Len(); i++ {
		// 			dataInStr[i] = slice.Index(i).Elem().String()
		// 		}
		// 		buff := new(bytes.Buffer)
		// 		wr := csv.NewWriter(buff)
		// 		wr.Write(dataInStr)
		// 		wr.Flush()
		// 		_ = cmd.Flags().Set(f.Name, buff.String())
		// 	} else {
		// 		_ = cmd.Flags().Set(f.Name, fmt.Sprintf("%v", flagVal))
		// 	}
		// }
	})
}

// ValueKind describes the pflag representation captured for a generated flag.
type ValueKind uint8

const (
	ValueKindUnknown ValueKind = iota
	ValueKindBool
	ValueKindInt
	ValueKindUint
	ValueKindInt16
	ValueKindUint16
	ValueKindInt32
	ValueKindUint32
	ValueKindInt64
	ValueKindUint64
	ValueKindFloat32
	ValueKindFloat64
	ValueKindString
	ValueKindDuration
	ValueKindBoolSlice
	ValueKindIntSlice
	ValueKindUintSlice
	ValueKindInt32Slice
	ValueKindInt64Slice
	ValueKindStringSlice
)

// Value is the immutable raw state of a pflag value.
type Value struct {
	Kind     ValueKind
	Type     string
	String   string
	DefValue string
	Changed  bool
}

// FlagValue is retained as an additive compatibility alias.
type FlagValue = Value

// EnvBinding records the environment variable associated with a flag.
type EnvBinding struct{ Name string }

type Field struct {
	Name  string
	Usage string
	Value Value
	Env   EnvBinding
}

// KeyValue is a key and its default representation.
type KeyValue struct {
	Key   string
	Value string
}

// Metadata is an immutable snapshot of local generated flags.
type Metadata struct{ fields []Field }

// Fields returns a deep copy of captured fields.
func (m Metadata) Fields() []Field {
	result := make([]Field, len(m.fields))
	copy(result, m.fields)
	return result
}

// CaptureMetadata captures local flags after Cobra parsing.
func CaptureMetadata(cmd *cobra.Command) (Metadata, error) {
	if cmd == nil {
		return Metadata{}, fmt.Errorf("vcflag: nil command")
	}
	fields := make([]Field, 0)
	var unsupported string
	cmd.LocalNonPersistentFlags().VisitAll(func(flag *pflag.Flag) {
		kind, ok := valueKind(flag.Value.Type())
		if !ok {
			unsupported = flag.Name
			return
		}
		env := ""
		if names := flag.Annotations["vcflag-env"]; len(names) > 0 {
			env = names[0]
		}
		fields = append(fields, Field{Name: flag.Name, Usage: flag.Usage, Value: Value{Kind: kind, Type: flag.Value.Type(), String: flag.Value.String(), DefValue: flag.DefValue, Changed: flag.Changed}, Env: EnvBinding{Name: env}})
	})
	if unsupported != "" {
		return Metadata{}, &UnsupportedValueError{Name: unsupported}
	}
	return Metadata{fields: fields}, nil
}

// UnsupportedValueError reports a pflag value that cannot be decoded safely.
type UnsupportedValueError struct{ Name string }

func (e *UnsupportedValueError) Error() string {
	return fmt.Sprintf("vcflag: unsupported flag value %q", e.Name)
}

func valueKind(typ string) (ValueKind, bool) {
	switch typ {
	case "bool":
		return ValueKindBool, true
	case "int":
		return ValueKindInt, true
	case "uint":
		return ValueKindUint, true
	case "int16":
		return ValueKindInt16, true
	case "uint16":
		return ValueKindUint16, true
	case "int32":
		return ValueKindInt32, true
	case "uint32":
		return ValueKindUint32, true
	case "int64":
		return ValueKindInt64, true
	case "uint64":
		return ValueKindUint64, true
	case "float32":
		return ValueKindFloat32, true
	case "float64":
		return ValueKindFloat64, true
	case "string":
		return ValueKindString, true
	case "duration":
		return ValueKindDuration, true
	case "boolSlice":
		return ValueKindBoolSlice, true
	case "intSlice":
		return ValueKindIntSlice, true
	case "uintSlice":
		return ValueKindUintSlice, true
	case "int32Slice":
		return ValueKindInt32Slice, true
	case "int64Slice":
		return ValueKindInt64Slice, true
	case "stringSlice":
		return ValueKindStringSlice, true
	default:
		return ValueKindUnknown, false
	}
}
