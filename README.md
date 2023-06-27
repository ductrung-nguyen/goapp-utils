# 1. What is this?
This is a repository containing some common ultilities to build a Golang application. It supports:
- Watching files and raising actions
- Logging in JSON or normal format in a performant way using uzap
- Building CLI tools with many parameters and binding them to a config struct
- Mocking HTTP requests
- Mocking K8s API results

# 2. Features
## 2.1. File watching

By default, the building watcher with fsnotify supports monitoring existing file until it's deleted. It doesn't support watching a file that doesn't exist, or it is deleted and re-creted again.

The module `filewatcher` in this repository solves all these limitations.

Example:

```go
package main

import (
	"context"
	"fmt"

	"github.com/fsnotify/fsnotify"
	"github.com/ductrung-nguyen/goapp-utils/pkg/filewatcher"
)

func main() {
	var handler = func(f *filewatcher.FileWatcher, event fsnotify.Event) {
		fmt.Printf("Receive event: %#v\n", event)
	}

	// watch file "my_file.txt" even if it does not exist yet
	fw, err := filewatcher.New("my_file.txt", handler, nil)
	if err == nil {
		fw.Watch(context.Background())
	}
}
```

The code above monitors file `my_file.txt`.
We can run the code with `go run main.go` in a Terminal tab. At this time, the monitored file does not exist. Please open another Terminal tab to create that file, do modification or delete the file. The application will print all events happened on that file.

## 2.2. Logging

Module `logger` provides a simple and efficient way to log events in Golang application using `uzap` and `logr`.

```go
package main

import (
	"errors"

	"github.com/ductrung-nguyen/goapp-utils/pkg/logger"
)

func main() {
	// Using logger with default configuration
	logger.Root.WithName("OptionalLoggerName").Info("This is a simple log at level 0")

	logger.Root.Info("This is a simple log at level 0 with some key-value pairs", "podName", "indexer-0", "No.", 1)
	logger.Root.WithValues("podName", "indexer-0", "No.", 1).Info("The same simple log at level 0 with some key-value pairs")
	logger.Root.Info("This is a simple log at level 1")
	logger.Root.V(2).Info("This message is not printed because its level =2, higher than the default max allowed level = 1")

	// if we want to change the configuration
	logger.InitLogger(&logger.LoggerConfig{
		Folder:       "logs", // where to store the log files
		Environment:  "prod", // or any other value to use Environment "development"
		Encoder:      "json", // or any other value to use encoder "console"
		LogToConsole: true,   // it will write logs to file and stdout
		Level:        3,      // The maximum level of the logs that can be printed
		MaxSizeInMB:  100,    // max size of each log file before rolling
		MaxAge:       10,     // max age of a log file
		Compress:     true,
	})

	logger.Root.V(2).Info("This message is printed as its level = 2, lower than max allowed level = 3")
	logger.Root.V(4).Info("This message is not printed anywhere as its level = 4, higher than max allowed level = 3")
	logger.Root.V(5).Error(errors.New("a dummy error"), "This error message is still printed even if its level is higher than the max allowed level")
}
```
When runnning that application with `go run main.go`, we got:
```bash
2023-06-28T01:00:37.288+0200	INFO	OptionalLoggerName	logging/main.go:11	This is a simple log at level 0
2023-06-28T01:00:37.290+0200	INFO	logging/main.go:13	This is a simple log at level 0 with some key-value pairs	{"podName": "indexer-0", "No.": 1}
2023-06-28T01:00:37.290+0200	INFO	logging/main.go:14	The same simple log at level 0 with some key-value pairs	{"podName": "indexer-0", "No.": 1}
2023-06-28T01:00:37.290+0200	INFO	logging/main.go:15	This is a simple log at level 1
{"level":"Level(-2)","ts":1687906837.2905312,"caller":"logging/main.go:30","msg":"This message is printed as its level = 2, lower than max allowed level = 3"}
{"level":"error","ts":1687906837.290842,"caller":"logging/main.go:32","msg":"This error message is still printed even if its level is higher than the max allowed level","error":"a dummy error"}
```

## 2.3 Parameters binding

When building CLI application that can handle different parameters, we can use either the building package `flag` or other 3rd party library.

For example:
```golang
package main

import (
	"flag"
	"fmt"
)

var (
	file        = flag.String("k8sconfig", "", "Path to K8s config file")
	namespace   = flag.String("namespace", "", "Namespace")
	count       = flag.Int("count", 2, "count params")
	repeat      = flag.Bool("repeat", false, "Repeat execution")
)

func main() {
	flag.Parse()

	fmt.Println("file name: ", *file)
	fmt.Println("Namespace: ", *namespace)
	fmt.Println("count: ", *count)
	fmt.Println("repeat: ", *repeat)
}
```
The above application defines 3 flags: "file", "count" and "repeat".

They works fine for simple cases. However, when we need to bind the parameters into a struct, for instance, a configuration struct, it can be more verbose.
And what if we want to support using parameters from environment variables?
The module `vcflag` is designed for that purpose. It uses package `viper` to read and store configuration in different ways: from CLI params, from file, from environment variables...

For example, our application has a struct Config to store the configurations.

```golang
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
	"github.com/ductrung-nguyen/goapp-utils/pkg/logger"
	"github.com/ductrung-nguyen/goapp-utils/pkg/vcflag"
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

```

When building using `go build` then executing the above application:
```bash
./vcflag -h
An application to show how can we use vcflag with viper and corba
	to build rich functionality CLI

Usage:
  vcflag [flags]

Flags:
      --Count int                          Overrided by Env Var DEMO_COUNT
      --K8sCfg.KubeConfigFilePath string   Overrided by Env Var DEMO_K8SCFG__KUBECONFIGFILEPATH
      --K8sCfg.Namespace string            Overrided by Env Var DEMO_K8SCFG__NAMESPACE
      --Logger.Compress                    Overrided by Env Var DEMO_LOGGER__COMPRESS
      --Logger.Encoder string              Overrided by Env Var DEMO_LOGGER__ENCODER
      --Logger.Environment string          Overrided by Env Var DEMO_LOGGER__ENVIRONMENT
      --Logger.Filename string             Overrided by Env Var DEMO_LOGGER__FILENAME
      --Logger.Folder string               Overrided by Env Var DEMO_LOGGER__FOLDER
      --Logger.Level int                   Overrided by Env Var DEMO_LOGGER__LEVEL
      --Logger.LogToConsole                Overrided by Env Var DEMO_LOGGER__LOGTOCONSOLE
      --Logger.MaxAge int                  Overrided by Env Var DEMO_LOGGER__MAXAGE
      --Logger.MaxBackups int              Overrided by Env Var DEMO_LOGGER__MAXBACKUPS
      --Logger.MaxSizeInMB int             Overrided by Env Var DEMO_LOGGER__MAXSIZEINMB
      --Logger.SkipCaller                  Overrided by Env Var DEMO_LOGGER__SKIPCALLER
      --Repeat                             Overrided by Env Var DEMO_REPEAT
      --config string                      config file
      --generate-empty-config              generate empty config file?
  -h, --help                               help for vcflag
```

We are able to use config from a yaml file from some default directories. If the application cannot find the config file in these folder, it will panic.
Or we can specify our configuration file through `--config <path_to_config_file>`.

We can also use environment variables to override the values, for example:
```bash
# override the log level to 3
DEMO_LOGGER__LEVEL=3 ./vcflag
```

To see how a config file is look like, please run `./vcflag --generate-empty-config`

## 2.4. HTTP Client

## 2.5. Kubernetes client