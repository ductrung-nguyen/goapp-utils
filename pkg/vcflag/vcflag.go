package vcflag

import (
	"fmt"
	"reflect"
	"strings"

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

// Init the config reader via Viper and Cobra objects.
// we can specify either the config file name, or the config name, and its type
// as well as the locations to find that config files
func InitConfigReader(
	viperObj *viper.Viper, cmd *cobra.Command,
	cfgFile string, cfgName string, cfgType string, configLocations []string,
	envPrefix string,
	logger *logr.Logger,
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

	return nil
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
func GenerateFlags(currentPath string, key string, value reflect.Value,
	viperObj *viper.Viper, command *cobra.Command) error {
	comment := ""
	if idx := strings.Index(key, ";"); idx >= 0 {
		comment = strings.Trim(key[idx+1:], " \t")
		key = strings.Trim(key[:idx], " \t")
	}

	path := key
	if currentPath != "" {
		path = fmt.Sprintf("%s.%s", currentPath, key)
	}
	s := value
	typeOfT := s.Type()
	switch value.Kind() {
	case reflect.Struct:
		for idx := 0; idx < typeOfT.NumField(); idx += 1 {
			tag := getStructTag(typeOfT.Field(idx), "pflag")

			// if there is no value of tag pflag, try to use value of tag `mapstructure`
			if tag == "" {
				tag = getStructTag(typeOfT.Field(idx), "mapstructure")
			}

			// if tag is "-", the user wants to skip this field
			if tag == "-" {
				return nil
			} else if tag == "" {
				// if tag is empty, try to use field name to generate flag
				tag = strings.ReplaceAll(typeOfT.Field(idx).Name, "-", "_")
				tag = strings.ReplaceAll(tag, ".", "__")
				tag = strings.ReplaceAll(tag, " ", "_")
				return GenerateFlags(path, tag, s.Field(idx), viperObj, command)
			}
		}
	case reflect.Int:
		command.Flags().Int(path, 0, comment)
		return viperObj.BindPFlag(path, command.Flags().Lookup(path))
	case reflect.String:
		command.Flags().String(path, "", comment)
		return viperObj.BindPFlag(path, command.Flags().Lookup(path))
	}

	return nil
}

// func getEnvName(flagName string) string {
// 	envVarSuffix := strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(flagName, "-", "_"), ".", "__"))
// 	return fmt.Sprintf("%s_%s", envPrefix, envVarSuffix)
// }

// Bind each cobra flag to its associated viper configuration (config file and environment variable)
func bindEnvVarsToFlags(cmd *cobra.Command, v *viper.Viper, envPrefix string, logger *logr.Logger) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		// Environment variables can't have dashes in them, so bind them to their equivalent
		// keys with underscores, e.g. --favorite-color to MYAPP_FAVORITE_COLOR
		if strings.Contains(f.Name, "-") || strings.Contains(f.Name, ".") {
			envVarSuffix := strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(f.Name, "-", "_"), ".", "__"))
			envName := fmt.Sprintf("%s_%s", envPrefix, envVarSuffix)
			logger.V(2).Info("Binding env to flag", "env", envName, "flag", f.Name)
			_ = v.BindEnv(f.Name, envName)
			if f.Usage != "" {
				f.Usage += ". "
			}
			f.Usage += "Overrided by Env Var " + envName
		}

		// Apply the viper config value to the flag when the flag is not set and viper has a value
		if !f.Changed && v.IsSet(f.Name) {
			val := v.Get(f.Name)
			_ = cmd.Flags().Set(f.Name, fmt.Sprintf("%v", val))
		}
	})
}
