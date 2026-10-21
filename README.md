# goapp-utils

[![Go](https://github.com/ndtrunk/goapp-utils/actions/workflows/go.yml/badge.svg)](https://github.com/ndtrunk/goapp-utils/actions/workflows/go.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/ndtrunk/goapp-utils.svg)](https://pkg.go.dev/github.com/ndtrunk/goapp-utils)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE.md)

Most Go services need the same supporting code: flags for each configuration field, environment overrides for deployment, structured logs, and a way to react when configuration changes. Writing that setup repeatedly takes time away from the application itself-and gives you more code to keep consistent and test.

**goapp-utils brings that setup together in reusable packages.** Define a configuration struct once, expose it through CLI flags, YAML, and environment variables, and add live reload when your service needs it. Use structured logging and file-watching helpers alongside your own application code, with client interfaces that let you test HTTP and Kubernetes integrations without production services.

You keep your existing application structure. Adopt one package at a time, and continue using the Cobra, Viper, Zap, logr, fsnotify, and client-go APIs you already know.

## What can it help with?

| When you need to… | Use | What you write less of |
| --- | --- | --- |
| Configure a CLI or service | [`bootstrap`](https://pkg.go.dev/github.com/ndtrunk/goapp-utils/pkg/bootstrap) | Flag registration, YAML loading, environment binding, and reload lifecycle management |
| Add generated flags to your own Cobra/Viper setup | [`vcflag`](https://pkg.go.dev/github.com/ndtrunk/goapp-utils/pkg/vcflag) | Repeated flag definitions for nested configuration fields |
| Log structured events | [`logger`](https://pkg.go.dev/github.com/ndtrunk/goapp-utils/pkg/logger) | Zap/logr setup, JSON or console encoding, and rotating log files |
| React to files appearing or being recreated | [`filewatcher`](https://pkg.go.dev/github.com/ndtrunk/goapp-utils/pkg/filewatcher) | Reattaching watches when a target disappears |
| Make HTTP calls or inject Kubernetes clients | [`utils`](https://pkg.go.dev/github.com/ndtrunk/goapp-utils/pkg/utils) | Request setup and client construction, with interfaces for test doubles |

## Install

Requires **Go 1.27 or newer**; use the latest patch release for security fixes. The file-watching code uses Unix-specific APIs; use a Unix-like development environment such as Linux or macOS.

From an existing Go module:

```sh
go get github.com/ndtrunk/goapp-utils
```

Import individual packages, for example `github.com/ndtrunk/goapp-utils/pkg/bootstrap`. If you used an earlier repository location, update your imports and `go.mod` to this path, then run `go mod tidy`.

## Quick start: one struct, CLI flags, YAML, and environment variables

Save this as `main.go`. It loads configuration once, prints the resolved values, and exits. No configuration file is required to start.

```go
package main

import (
	"fmt"
	"os"

	"github.com/ndtrunk/goapp-utils/pkg/bootstrap"
	"github.com/spf13/cobra"
)

// Keep decoding keys and generated flag names (--name and --count) consistent.
type Config struct {
	Name  string `mapstructure:"name" yaml:"name" usage:"Name to greet"`
	Count int    `mapstructure:"count" yaml:"count" usage:"Number of greetings"`
}

func run() error {
	// Look for config.yaml locally and bind HELLO_NAME / HELLO_COUNT overrides.
	manager, err := bootstrap.New(bootstrap.Options{
		CommandUse:    "hello",
		SearchPaths:   []string{"."},
		MissingPolicy: bootstrap.MissingConfigAllowed,
		EnableEnv:     true,
		EnvPrefix:     "HELLO",
	}, Config{})
	if err != nil {
		return err
	}

	// Generate the Cobra command and its flags from the Config type.
	cmd, err := manager.NewCommand()
	if err != nil {
		return err
	}
	// RunE runs after flag parsing, so Load can apply CLI, environment, and YAML values.
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		var cfg Config
		if err := manager.Load(cmd, &cfg); err != nil {
			return err
		}
		fmt.Printf("name=%s count=%d\n", cfg.Name, cfg.Count)
		for i := 0; i < cfg.Count; i++ {
			fmt.Printf("Hello, %s!\n", cfg.Name)
		}
		return nil
	}
	// Parse arguments and invoke the configuration-loading handler above.
	return cmd.Execute()
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

Try flags and environment variables immediately:

```sh
go run . --help
go run . --name Ada --count 2
HELLO_NAME=Grace go run . --count 1
```

Or add `config.yaml` in the same directory:

```yaml
name: Ada
count: 2
```

```sh
go run .                              # name=Ada count=2
HELLO_COUNT=3 go run .                 # name=Ada count=3
HELLO_COUNT=3 go run . --count 1        # name=Ada count=1
go run . --config config.yaml --count 1
```

**Precedence:** explicitly supplied flags override environment variables, which override YAML values, which override generated zero-value defaults.

Configuration details worth knowing:

- `mapstructure` tags give fields predictable lowercase flag names and decoding keys. `pflag` tags override flag names; without either tag, flags use the Go field name, including its case. `yaml` tags alone do not rename flags.
- Nested fields produce dotted flags, such as `--server.port`; environment names replace dots with underscores, such as `HELLO_SERVER_PORT`.
- `Config{}` is a type prototype, not a source of runtime defaults. Nonzero values passed to `bootstrap.New` are not retained. Put application values in YAML, environment variables, or flags.
- The default file name is `config.yaml`; `SearchPaths` selects where to look. Set `MissingConfigRequired` to require a file. An explicitly selected `--config` file must exist even with `MissingConfigAllowed`.
- Call `manager.Load` after Cobra parses flags, using that manager's command and a pointer to the prototype's type. Each manager supports one load attempt. Loading does not start a watcher or initialize logging.

Already own your Cobra and Viper instances? Use [`vcflag.GenerateFlags`](https://pkg.go.dev/github.com/ndtrunk/goapp-utils/pkg/vcflag#GenerateFlags) to register generated flags on your command instead of adopting the bootstrap manager.

## Reload configuration without restarting

For long-running services, use `bootstrap.NewReloadRunE` instead of the one-shot `manager.Load` handler. It captures parsed flags, prepares the YAML decoder, starts the reload manager, waits for cancellation, and closes it.

The complete [reload example](examples/vcflag/main.go) includes:

- A nested configuration struct and generated flags.
- YAML and `DEMO_` environment overrides.
- A subscriber that prints the initial configuration and subsequent changes.
- A caller-owned signal context for shutdown with Ctrl+C or SIGTERM.

Run it from a clone of this repository:

```sh
git clone https://github.com/ndtrunk/goapp-utils.git
cd goapp-utils
go run ./examples/vcflag --help
go run ./examples/vcflag
```

While it runs, create or edit `config.yaml` in the repository root:

```yaml
count: 2
repeat: true
k8sCfg:
  namespace: default
```

The subscriber prints configuration events as the file changes. To try an environment override or an explicit flag, restart with `DEMO_COUNT=3 go run ./examples/vcflag --Repeat`; the example's untagged Go fields generate `--Count` and `--Repeat`, not lowercase flags.

The nested YAML key is `k8sCfg`, matching the Go field used by Viper; the example's `yaml` tag does not control Viper decoding. Press Ctrl+C to stop. The current example reports `context canceled` on shutdown; applications can handle that cancellation at their own command boundary.

When integrating reload into your application:

- Assign the returned handler to the manager-owned command's `RunE`, then call `ExecuteContext` yourself. `NewReloadRunE` does not execute Cobra or install signal handlers. Create managed subcommands through `manager.NewSubcommand`, not a separate Cobra command.
- Supply a `Clone` function in `ReloadOptions`. A value copy is sufficient for the example's scalar/nested-value fields; configurations containing slices, maps, or pointers need a deep copy to keep snapshots isolated.
- Use `Validate` to reject invalid updates and `OnError` to observe reload errors. The default deletion policy is `RetainLastGood`; `Clear` and `Stop` are also available.
- Explicit CLI values remain overrides during reload; environment variables are read again when decoding. Logger initialization remains a separate, opt-in step.

## Structured logging with rotation

Use named loggers and key/value fields without repeating encoder and output setup:

```go
package main

import "github.com/ndtrunk/goapp-utils/pkg/logger"

func main() {
	// Replace the default logger with JSON output to both console and rotating files.
	logger.InitLogger(&logger.LoggerConfig{
		Environment:  "prod",
		Encoder:      "json",
		Folder:       "logs",
		Filename:     "app.log",
		LogToConsole: true,
		Level:        1,
		MaxSizeInMB:  100,
		MaxBackups:   3,
		MaxAge:       7,
		Compress:     true,
	})

	// Attach shared context once; every event from this logger includes the queue.
	log := logger.Root.WithName("worker").WithValues("queue", "emails")
	log.Info("Worker started", "concurrency", 4)
	// V(1) is enabled by Level: 1; higher-verbosity messages would be filtered out.
	log.V(1).Info("Processing batch", "size", 20)
}
```

`Encoder: "json"` selects JSON; other values select console encoding. `Environment: "prod"` selects Zap's production encoder settings. `Level` is the maximum logr verbosity: `Level: 1` enables `V(0)` and `V(1)`, but not `V(2)`. Error logs are not filtered by verbosity.

Importing `logger` initializes `logger.Root` with default file and console logging. Call `InitLogger` to replace those defaults; provide an explicit folder and filename for file output. Relative log file paths are resolved against the executable's directory, not necessarily your shell's working directory. See the [logging example](examples/logging/main.go) for more logging patterns.

## Watch files that may not exist yet

`ReloadWatcher` watches the nearest existing parent while a target is absent and reattaches when it appears. It also supports deletion and recreation, context cancellation, and explicit close.

Save this as `main.go` in a separate example module:

```go
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/ndtrunk/goapp-utils/pkg/filewatcher"
	"github.com/ndtrunk/goapp-utils/pkg/watchapi"
)

func main() {
	// Ctrl+C cancels the context and stops the watcher.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Watch the parent until settings.yaml exists, then follow the file itself.
	watcher, err := filewatcher.NewReloadWatcher(ctx, []watchapi.Target{
		{ConfiguredPath: "settings.yaml"},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	// Release filesystem watches and background resources when main returns.
	defer watcher.Close()

	// Consume file events and watcher errors until cancellation or channel closure.
	for {
		select {
		case event, ok := <-watcher.Events():
			if !ok {
				return
			}
			fmt.Printf("file event: %+v\n", event)
		case err, ok := <-watcher.Errors():
			if !ok {
				return
			}
			fmt.Fprintln(os.Stderr, err)
		case <-ctx.Done():
			return
		}
	}
}
```

Run `go run .`, then create, edit, remove, and recreate `settings.yaml` from another terminal. Filesystem operations can produce multiple events; use bootstrap's reload lifecycle when you need configuration decoding, validation, and debouncing rather than raw notifications.

The callback-based `filewatcher.New` API is also demonstrated in [examples/filewatcher](examples/filewatcher/main.go). Prefer `NewReloadWatcher` for new code that needs explicit lifecycle control.

## HTTP and Kubernetes helpers

### HTTP requests with an injectable client

Use `RealHTTPClient` directly, or accept `HttpClientInterface` in application code so tests can substitute their own implementation. This function adds a request timeout, keeps TLS certificate verification enabled, and treats non-200 responses as application errors:

```go
package health

import (
	"fmt"
	"net/http"
	"time"

	"github.com/ndtrunk/goapp-utils/pkg/utils"
)

// Accept the interface so callers can inject a real client or a test double.
func FetchHealth(client utils.HttpClientInterface, url string) ([]byte, error) {
	body, status, err := client.SendRequest(
		url, nil, nil, http.MethodGet, nil, nil,
		// Keep TLS verification enabled, omit basic auth, and use a five-second timeout.
		utils.NewRequestOptions(false, utils.HttpBasicAuth{}, 5*time.Second),
	)
	if err != nil {
		return nil, err
	}
	// A successful HTTP exchange can still carry an unsuccessful status code.
	if status != http.StatusOK {
		return nil, fmt.Errorf("health endpoint returned HTTP %d", status)
	}
	return body, nil
}
```

Call it with `FetchHealth(utils.RealHTTPClient{}, "https://your-service.example/health")`, using your service's URL. Requests support headers, query parameters, cookies, basic authentication, and gzip responses. `StreamRequest` returns a reader that the caller must close. HTTP error status codes do not automatically become Go errors; check the returned status as above.

### Kubernetes clients without hard-wiring construction

`utils.K8sClientHelper{}.GetClient("/path/to/kubeconfig")` constructs a client from kubeconfig; an empty path selects in-cluster service-account configuration. `GetClientAndConfig` also returns the underlying `rest.Config`.

Accept `utils.IK8sClientHelper` where your application constructs clients, or pass a `utils.K8sClient` directly. Its `Clientset` field is a `kubernetes.Interface`, so tests can supply client-go's fake client without a live cluster:

```go
package cluster

import (
	"context"

	"github.com/ndtrunk/goapp-utils/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Accept an injected client; tests can supply client-go's fake Clientset.
func PodNames(ctx context.Context, client *utils.K8sClient, namespace string) ([]string, error) {
	// Forward the caller's context so cancellation also cancels the Kubernetes request.
	pods, err := client.Clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(pods.Items))
	for _, pod := range pods.Items {
		names = append(names, pod.Name)
	}
	return names, nil
}
```

These are injection points, not a bundled mocking framework. Use your own test doubles or client-go fakes according to what your application needs to verify.

## Contributing

Found a bug or have a useful helper to share? See [CONTRIBUTING.md](CONTRIBUTING.md) for development checks, pull request guidance, and release conventions.

## License

[MIT](LICENSE.md).
