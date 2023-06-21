// Copyright (c) 2022-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package service

import (
	"fmt"
	"regexp"

	recorder "github.com/mattermost/calls-recorder/cmd/recorder/config"
)

type JobType string

const (
	JobTypeRecording            JobType = "recording"
	minSupportedRecorderVersion         = "0.3.3"
)

// We currently support two formats, semantic version tag or image hash (sha256).
// TODO: Consider deprecating tag version and switch to hash only.
var recorderRunnerREs = []*regexp.Regexp{
	regexp.MustCompile(`^mattermost/calls-recorder@sha256:\w{64}$`),
	regexp.MustCompile(`^mattermost/calls-recorder:v((?:0|[1-9]\d*)\.(?:0|[1-9]\d*)\.(?:0|[1-9]\d*))$`),
}

type Job struct {
	JobConfig
	ID         string         `json:"id"`
	StartAt    int64          `json:"start_at"`
	StopAt     int64          `json:"stop_at,omitempty"`
	OutputData map[string]any `json:"output_data,omitempty"`
}

type JobConfig struct {
	Type           JobType        `json:"type"`
	MaxDurationSec int64          `json:"max_duration_sec"`
	Runner         string         `json:"runner"`
	InputData      map[string]any `json:"input_data,omitempty"`
}

func JobRunnerIsValid(runner string) error {
	for _, re := range recorderRunnerREs {
		if matches := re.FindStringSubmatch(runner); len(matches) > 1 {
			if err := checkMinVersion(minSupportedRecorderVersion, matches[1]); err != nil {
				return err
			}
			return nil
		}
	}
	return fmt.Errorf("failed to validate runner")
}

func (c JobConfig) IsValid() error {
	if c.Type == "" {
		return fmt.Errorf("invalid Type value: should not be empty")
	}

	if c.Runner == "" {
		return fmt.Errorf("invalid Runner value: should not be empty")
	}

	switch c.Type {
	case JobTypeRecording:
		if err := JobRunnerIsValid(c.Runner); err != nil {
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

	if c.MaxDurationSec < 0 {
		return fmt.Errorf("invalid MaxDurationSec value: should not be negative")
	}

	return nil
}
