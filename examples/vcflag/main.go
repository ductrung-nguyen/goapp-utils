package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/ductrung-nguyen/goapp-utils/pkg/logger"
	"github.com/ductrung-nguyen/goapp-utils/pkg/vcflag"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
)

type K8sConfig struct {
	KubeConfigFilePath string `yaml:"kubeConfigPath"`
	Namespace          string `yaml:"namespace"`
}

type Config struct {
	K8sCfg      K8sConfig `yaml:"k8sConfig"`
	Count       int       `yaml:"count" pflag:"count"`
	Repeat      bool      `yaml:"repeat" flag:"repeat"`
	NoUseInFlag int       `pflag:"-"`

	// Logger configuration
	Logger logger.LoggerConfig `yaml:"logger"`
}

var configManager *viper.Viper
var cfgFile string           // allow user to specify the config file in a custom path
var generateEmptyConfig bool // should we generate empty config file?

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "vcflag",
	Short: "A simple application to demo vcflag",
	Long: `An application to show how can we use vcflag with viper and corba
	to build rich functionality CLI`,

	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// This function will be run before the main logic execution
		if generateEmptyConfig {
			b, err := yaml.Marshal(Config{})
			os.WriteFile("config.yaml", b, os.ModePerm)
			return err
		}
		// before running the command, we need to setup the config manager
		// to ask it to look at the configuration file in different directories
		return setupConfigManager(configManager, "config", cmd, args)
	},

	Run: func(cmd *cobra.Command, args []string) {
		logger.Root.Info("Starting the main logic of the command here")
		currentConfig, _ := getConfigFromManager(configManager)
		logger.Root.Info("We can use the config object", "config", currentConfig)
	},
}

// this function is executed automatically whenever we use package main
// That means, it will be executed first ( before the global variables delaration)
func init() {
	configManager = viper.New()

	// allow user to specify the config file in any custom location
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file")
	rootCmd.PersistentFlags().BoolVar(&generateEmptyConfig, "generate-empty-config", false, "generate empty config file?")
	// generate flags from config struct
	// to allow us override configuration from the command line
	if err := vcflag.GenerateFlags(Config{}, configManager, rootCmd); err != nil {
		return
	}

	// allow user to use environment variable to override the parameters (flags))
	vcflag.BindEnvVarsToFlags(configManager, rootCmd, "DEMO", &logger.Root)
}

func main() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

// setupConfigManager configures the configuration manager by setting up folders that can contain configuration files
// and watch that configuration file
// the configuration file parameter should not contain the extension. For example, if the file is "config.yaml"
// the `configFileName` should be `config`
// Here we only support extension `yaml`
// if watchDebuggingFile is true, we allow to use "configFileName"-debug.yaml to override the main configuration file
// This option is used mainly for testing purpose, espcially when we deploy the application in K8s through ArgoCD,
// and we are not able to modify the main configuration file easily. In that case, we only need to create new configmaps
// and mount it as the debugging configuration file to override the default configurations
func setupConfigManager(cfgManager *viper.Viper, configFileName string, cmd *cobra.Command, args []string) error {

	// look for configuration file containing command name by order of the lower priority:
	// first ./configs/yaml, then ./config.yaml, and then $HOME/.vcflag/config.yaml
	configLocations := []string{"./configs/" + cmd.Name(), "./configs", ".", fmt.Sprintf("$HOME/.%s", cmd.Root().Name())}
	if err := vcflag.InitConfigReader(
		cfgManager, cmd, cfgFile, configFileName, "yaml",
		configLocations, strings.ToUpper(cmd.Name()), &logger.Root, true,
	); err != nil {
		return err
	}
	cfgManager.WatchConfig()

	return nil
}

// getConfigFromManager returns the configuration object from viper object
// Note that viper takes the config from files, environment variables, and CLI flags
func getConfigFromManager(confManager *viper.Viper) (*Config, error) {
	conf := &Config{}

	if len(confManager.AllSettings()) == 0 {
		return nil, nil
	}
	err := confManager.Unmarshal(conf)
	if err != nil {
		logger.Root.WithName("CFG").Error(err, "unable to decode into config struct")
		return nil, err
	}
	return conf, nil
}
