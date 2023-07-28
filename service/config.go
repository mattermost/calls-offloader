// Copyright (c) 2022-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package service

import (
	"fmt"

	"github.com/mattermost/calls-offloader/logger"
	"github.com/mattermost/calls-offloader/service/api"
	"github.com/mattermost/calls-offloader/service/auth"
)

type SecurityConfig struct {
	// Whether or not to enable admin API access.
	EnableAdmin bool `toml:"enable_admin"`
	// The secret key used to authenticate admin requests.
	AdminSecretKey string `toml:"admin_secret_key"`
	// Whether or not to allow clients to self-register.
	AllowSelfRegistration bool                    `toml:"allow_self_registration"`
	SessionCache          auth.SessionCacheConfig `toml:"session_cache"`
}

func (c SecurityConfig) IsValid() error {
	if !c.EnableAdmin {
		return nil
	}

	if c.AdminSecretKey == "" {
		return fmt.Errorf("invalid AdminSecretKey value: should not be empty")
	}

	return nil
}

type APIConfig struct {
	HTTP     api.Config     `toml:"http"`
	Security SecurityConfig `toml:"security"`
}

func (c APIConfig) IsValid() error {
	if err := c.Security.IsValid(); err != nil {
		return fmt.Errorf("failed to validate security config: %w", err)
	}

	if err := c.HTTP.IsValid(); err != nil {
		return fmt.Errorf("failed to validate http config: %w", err)
	}

	return nil
}

type StoreConfig struct {
	DataSource string `toml:"data_source"`
}

func (c StoreConfig) IsValid() error {
	if c.DataSource == "" {
		return fmt.Errorf("invalid DataSource value: should not be empty")
	}
	return nil
}

type JobAPIType string

const (
	JobAPITypeDocker     = "docker"
	JobAPITypeKubernetes = "kubernetes"
)

type JobsConfig struct {
	APIType           JobAPIType `toml:"api_type"`
	MaxConcurrentJobs int        `toml:"max_concurrent_jobs"`
}

func (c JobsConfig) IsValid() error {
	if c.APIType != JobAPITypeDocker && c.APIType != JobAPITypeKubernetes {
		return fmt.Errorf("invalid APIType value: %s", c.APIType)
	}

	if c.MaxConcurrentJobs <= 0 {
		return fmt.Errorf("invalid MaxConcurrentJobs value: should be greater than zero")
	}

	return nil
}

type Config struct {
	API    APIConfig
	Store  StoreConfig
	Jobs   JobsConfig
	Logger logger.Config
}

func (c Config) IsValid() error {
	if err := c.API.IsValid(); err != nil {
		return err
	}

	if err := c.Store.IsValid(); err != nil {
		return err
	}

	if err := c.Jobs.IsValid(); err != nil {
		return err
	}

	return c.Logger.IsValid()
}

func (c *Config) SetDefaults() {
	c.API.HTTP.ListenAddress = ":4545"
	c.API.Security.SessionCache.ExpirationMinutes = 1440
	c.Store.DataSource = "/tmp/calls-offloader-db"
	c.Jobs.APIType = JobAPITypeDocker
	c.Jobs.MaxConcurrentJobs = 2
	c.Logger.EnableConsole = true
	c.Logger.ConsoleJSON = false
	c.Logger.ConsoleLevel = "INFO"
	c.Logger.EnableFile = true
	c.Logger.FileJSON = true
	c.Logger.FileLocation = "calls-offloader.log"
	c.Logger.FileLevel = "DEBUG"
	c.Logger.EnableColor = false
}
