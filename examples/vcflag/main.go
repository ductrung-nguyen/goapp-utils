package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/ductrung-nguyen/goapp-utils/pkg/bootstrap"
	"github.com/ductrung-nguyen/goapp-utils/pkg/logger"
	"github.com/ductrung-nguyen/goapp-utils/pkg/vcflag"
)

type K8sConfig struct {
	KubeConfigFilePath string `yaml:"kubeConfigPath"`
	Namespace          string `yaml:"namespace"`
}

type Config struct {
	K8sCfg K8sConfig `yaml:"k8sConfig"`
	Count  int       `yaml:"count"`
	Repeat bool      `yaml:"repeat"`

	// Logger is optional application configuration; bootstrap does not initialize it.
	Logger logger.LoggerConfig `yaml:"logger"`
}

func main() {
	manager, err := bootstrap.New(bootstrap.Options{
		CommandUse:    "vcflag",
		ConfigName:    "config",
		SearchPaths:   []string{"./configs", "."},
		MissingPolicy: bootstrap.MissingConfigAllowed,
		EnableEnv:     true,
		EnvPrefix:     "DEMO",
	}, Config{})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	root, err := manager.NewCommand()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	root.Short = "A simple application to demo bootstrap and vcflag"
	runE, err := bootstrap.NewReloadRunE(
		manager,
		func() Config { return Config{} },
		func(raw vcflag.Metadata, schema bootstrap.PrototypeSchema, defaults []byte) (bootstrap.ReloadDecoderFactory[Config], error) {
			return bootstrap.NewYAMLReloadDecoderFactory[Config](raw, schema, defaults)
		},
		bootstrap.ReloadOptions[Config]{
			Clone: func(config Config) Config { return config },
			Subscribers: []bootstrap.Subscriber[Config]{func(config Config, event bootstrap.ReloadEvent[Config]) {
				fmt.Printf("config event %v: %+v\n", event.Kind, config)
				logger.Root.Info("Configuration changed", "kind", event.Kind, "path", event.Path, "config", config)
			}},
		},
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	root.RunE = runE

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := root.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
