// Copyright (c) 2022-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package job

import (
	"fmt"
	"regexp"

	recorder "github.com/mattermost/calls-recorder/cmd/recorder/config"
)

type Type string

const (
	TypeRecording Type = "recording"
)

const minSupportedRecorderVersion = "0.4.1"

// We currently support two formats, semantic version tag or image hash (sha256).
// TODO: Consider deprecating tag version and switch to hash only.
var recorderRunnerREs = []*regexp.Regexp{
	regexp.MustCompile(`^mattermost/calls-recorder@sha256:\w{64}$`),
	regexp.MustCompile(`^mattermost/calls-recorder:v((?:0|[1-9]\d*)\.(?:0|[1-9]\d*)\.(?:0|[1-9]\d*))$`),
	regexp.MustCompile(`^mattermost/calls-recorder-daily:v((?:0|[1-9]\d*)\.(?:0|[1-9]\d*)\.(?:0|[1-9]\d*))-dev$`),
}

type ServiceConfig struct {
	Runner string
}

type Job struct {
	Config
	ID         string         `json:"id"`
	StartAt    int64          `json:"start_at"`
	StopAt     int64          `json:"stop_at,omitempty"`
	OutputData map[string]any `json:"output_data,omitempty"`
}

type Config struct {
	Type           Type           `json:"type"`
	MaxDurationSec int64          `json:"max_duration_sec"`
	Runner         string         `json:"runner"`
	InputData      map[string]any `json:"input_data,omitempty"`
}

type StopCb func(job Job, success bool) error

func (c ServiceConfig) IsValid() error {
	return RunnerIsValid(c.Runner)
}

func RunnerIsValid(runner string) error {
	for _, re := range recorderRunnerREs {
		if matches := re.FindStringSubmatch(runner); len(matches) > 1 {
			return checkMinVersion(minSupportedRecorderVersion, matches[1])
		}
	}
	return fmt.Errorf("failed to validate runner")
}

func (c Config) IsValid() error {
	if c.Type == "" {
		return fmt.Errorf("invalid Type value: should not be empty")
	}

	if c.Runner == "" {
		return fmt.Errorf("invalid Runner value: should not be empty")
	}

	switch c.Type {
	case TypeRecording:
		if err := RunnerIsValid(c.Runner); err != nil {
			return fmt.Errorf("invalid Runner value: %w", err)
		}

		cfg := (&recorder.RecorderConfig{}).FromMap(c.InputData)
		cfg.SetDefaults()
		if err := cfg.IsValid(); err != nil {
			return fmt.Errorf("failed to validate InputData: %w", err)
		}
	default:
		return fmt.Errorf("invalid Type value: %q", c.Type)
	}

	if c.MaxDurationSec <= 0 {
		return fmt.Errorf("invalid MaxDurationSec value: should be positive")
	}

	return nil
}
