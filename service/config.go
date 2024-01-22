// Copyright (c) 2022-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package service

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/mattermost/calls-offloader/logger"
	"github.com/mattermost/calls-offloader/public/job"
	"github.com/mattermost/calls-offloader/service/api"
	"github.com/mattermost/calls-offloader/service/auth"
	"github.com/mattermost/calls-offloader/service/docker"
	"github.com/mattermost/calls-offloader/service/kubernetes"

	"github.com/kelseyhightower/envconfig"
)

var (
	retentionTimeRE = regexp.MustCompile(`^(\d+)([mhd])$`)
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
	JobAPITypeDocker     JobAPIType = "docker"
	JobAPITypeKubernetes            = "kubernetes"
)

// Alias is needed to implement custom unmarshaler.
type RetentionTime time.Duration

func (rt *RetentionTime) UnmarshalText(data []byte) error {
	d, err := parseRetentionTime(string(data))
	if err != nil {
		return err
	}
	*rt = RetentionTime(d)
	return nil
}

type JobsConfig struct {
	APIType                 JobAPIType                  `toml:"api_type"`
	MaxConcurrentJobs       int                         `toml:"max_concurrent_jobs"`
	FailedJobsRetentionTime RetentionTime               `toml:"failed_jobs_retention_time" ignored:"true"`
	ImageRegistry           string                      `toml:"image_registry"`
	Kubernetes              kubernetes.JobServiceConfig `toml:"kubernetes"`
	Docker                  docker.JobServiceConfig     `toml:"docker"`
}

// We need some custom parsing since duration doesn't support days.
func parseRetentionTime(val string) (time.Duration, error) {
	// Validate against expected format
	matches := retentionTimeRE.FindStringSubmatch(val)
	if len(matches) != 3 {
		return 0, fmt.Errorf("invalid retention time format")
	}

	// Parse days into duration
	if matches[2] == "d" {
		numDays, err := strconv.Atoi(matches[1])
		if err != nil {
			return 0, err
		}
		return time.Hour * 24 * time.Duration(numDays), nil
	}

	// Fallback to native duration parsing for anything else
	d, err := time.ParseDuration(val)
	if err != nil {
		return 0, err
	}

	return d, nil
}

func (c JobsConfig) IsValid() error {
	if c.APIType != JobAPITypeDocker && c.APIType != JobAPITypeKubernetes {
		return fmt.Errorf("invalid APIType value: %s", c.APIType)
	}

	if c.MaxConcurrentJobs <= 0 {
		return fmt.Errorf("invalid MaxConcurrentJobs value: should be greater than zero")
	}

	if c.FailedJobsRetentionTime < 0 {
		return fmt.Errorf("invalid FailedJobsRetentionTime value: should be a positive duration")
	}

	if c.FailedJobsRetentionTime > 0 && time.Duration(c.FailedJobsRetentionTime) < time.Minute {
		return fmt.Errorf("invalid FailedJobsRetentionTime value: should be at least one minute")
	}

	switch c.APIType {
	case JobAPITypeDocker:
		return c.Docker.IsValid()
	case JobAPITypeKubernetes:
		return c.Kubernetes.IsValid()
	}

	return nil
}

type Config struct {
	API    APIConfig
	Store  StoreConfig
	Jobs   JobsConfig
	Logger logger.Config
}

func (c *Config) ParseFromEnv() error {
	if val := os.Getenv("JOBS_FAILEDJOBSRETENTIONTIME"); val != "" {
		d, err := parseRetentionTime(val)
		if err != nil {
			return fmt.Errorf("failed to parse FailedJobsRetentionTime: %w", err)
		}
		c.Jobs.FailedJobsRetentionTime = RetentionTime(d)
	}

	return envconfig.Process("", c)
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
	c.Jobs.ImageRegistry = job.ImageRegistryDefault
	c.Logger.EnableConsole = true
	c.Logger.ConsoleJSON = false
	c.Logger.ConsoleLevel = "INFO"
	c.Logger.EnableFile = true
	c.Logger.FileJSON = true
	c.Logger.FileLocation = "calls-offloader.log"
	c.Logger.FileLevel = "DEBUG"
	c.Logger.EnableColor = false
}
