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
```go
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
The above application defines 3 flags: `file`, `count` and `repeat`.

They works fine for simple cases. However, when we need to bind the parameters into a struct, for instance, a configuration struct, it can be more verbose.
And what if we want to support using parameters from environment variables?
The module `vcflag` is designed for that purpose. It uses package `viper` to read and store configuration in different ways: from CLI params, from file, from environment variables...

For applications with a configuration struct, the `bootstrap` package provides a reload-capable lifecycle around `vcflag`. Pass a config prototype to `bootstrap.New`, create the Cobra command, then assign the handler returned by `bootstrap.NewReloadRunE`. The prototype is used to generate flags and establish the output type; values are decoded into a separate value of the same type.

```go
package main

import (
    "context"
    "fmt"
    "os"
    "os/signal"
    "syscall"

    "github.com/ductrung-nguyen/goapp-utils/pkg/bootstrap"
    "github.com/ductrung-nguyen/goapp-utils/pkg/vcflag"
)

type Config struct {
    Count  int  `yaml:"count"`
    Repeat bool `yaml:"repeat"`
}

func main() {
    manager, err := bootstrap.New(bootstrap.Options{
        CommandUse: "myapp", ConfigName: "config", SearchPaths: []string{"./configs", "."},
        MissingPolicy: bootstrap.MissingConfigAllowed, EnableEnv: true, EnvPrefix: "MYAPP",
    }, Config{})
    if err != nil { fmt.Fprintln(os.Stderr, err); os.Exit(1) }

    root, err := manager.NewCommand()
    if err != nil { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
    root.RunE, err = bootstrap.NewReloadRunE(
        manager,
        func() Config { return Config{} },
        func(raw vcflag.Metadata, schema bootstrap.PrototypeSchema, defaults []byte) (bootstrap.ReloadDecoderFactory[Config], error) {
            return bootstrap.NewYAMLReloadDecoderFactory[Config](raw, schema, defaults)
        },
        bootstrap.ReloadOptions[Config]{Clone: func(v Config) Config { return v }},
    )
    if err != nil { fmt.Fprintln(os.Stderr, err); os.Exit(1) }

    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()
    if err := root.ExecuteContext(ctx); err != nil { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
}
```

`NewReloadRunE` returns a Cobra `RunE` handler; it does not execute Cobra, install signal handlers, or create/own the process context. The caller must attach the returned handler to the command that owns the manager (for example, `root.RunE = runE`) and must call `ExecuteContext` with the caller's context, including any signal handling. Use the same manager-owned command boundary for subcommands: create the subcommand through that manager's `NewSubcommand` flow and attach the handler to that command, rather than attaching it to a foreign or separately constructed Cobra command. The handler captures the parsed Cobra metadata, prepares and builds the reload registration, starts the `ReloadManager`, waits for its context, and closes it. Its factory builder receives the captured metadata, prototype schema, and defaults, so custom config decoding remains possible. Pass `ReloadOptions.Subscribers` when the application needs initial/update/deletion/failure events. The complete migrated example is in `examples/vcflag/main.go`.

For applications that only need a one-shot load, `manager.Load(cmd, new(Config))` remains supported after Cobra parses flags. It does not start a watcher. The older manual capture/prepare/build lifecycle remains a legacy compatibility path, but new integrations should use `NewReloadRunE`.

Logger setup remains opt-in. When enabling logging configuration, provide the supported fields explicitly (as in the logging example), for example `logger.InitLogger(&logger.LoggerConfig{Folder: "logs", Filename: "app.log", LogToConsole: true, Level: 3, MaxSizeInMB: 100, MaxBackups: 2, MaxAge: 10, Compress: true})`. Do not pass an empty `LoggerConfig`, because it has no output sink.

Configuration files are YAML (`config.yaml` by default) and are searched in the configured `SearchPaths`. Set `MissingPolicy: bootstrap.MissingConfigRequired` when a file is mandatory. With environment binding enabled, the generated flag for the `Count` field is `--Count` (the field has no `pflag` or `mapstructure` name tag); it can be overridden by `MYAPP_COUNT`.

## 2.4. HTTP Client
## 2.5. Kubernetes client
