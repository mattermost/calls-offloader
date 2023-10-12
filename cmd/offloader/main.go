// Copyright (c) 2022-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/mattermost/calls-offloader/service"

	"github.com/BurntSushi/toml"
)

// loadConfig reads the config file and returns a new Config,
// This method overrides values in the file if there is any environment
// variables corresponding to a specific setting.
func loadConfig(path string) (service.Config, error) {
	var cfg service.Config
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		log.Printf("config file not found at %s, using defaults", path)
		cfg.SetDefaults()
	} else if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return cfg, fmt.Errorf("failed to decode config file: %w", err)
	}

	if err := cfg.ParseFromEnv(); err != nil {
		return cfg, err
	}

	return cfg, nil
}

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "config/config.toml", "Path to the configuration file for the calls-offloader service.")
	flag.Parse()

	cfg, err := loadConfig(configPath)
	if err != nil {
		log.Fatalf("calls-offloader: failed to load config: %s", err.Error())
	}

	if err := cfg.IsValid(); err != nil {
		log.Fatalf("calls-offloader: failed to validate config: %s", err.Error())
	}

	service, err := service.New(cfg)
	if err != nil {
		log.Fatalf("calls-offloader: failed to create service: %s", err.Error())
	}

	if err := service.Start(); err != nil {
		log.Fatalf("calls-offloader: failed to start service: %s", err.Error())
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	if err := service.Stop(); err != nil {
		log.Fatalf("calls-offloader: failed to stop service: %s", err.Error())
	}
}
