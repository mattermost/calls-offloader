// Copyright (c) 2022-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package service

import (
	"fmt"
	"regexp"
)

type JobType string

const (
	JobTypeRecording JobType = "recording"
)

// We currently support two formats, semantic version tag or image hash (sha256).
// TODO: Consider deprecating tag version and switch to hash only.
var recorderRunnerREs = []*regexp.Regexp{
	regexp.MustCompile(`^mattermost/calls-recorder@sha256:\w{64}$`),
	regexp.MustCompile(`^mattermost/calls-recorder:v(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$`),
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

func JobRunnerIsValid(runner string) bool {
	for _, re := range recorderRunnerREs {
		if re.MatchString(runner) {
			return true
		}
	}
	return false
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
		if !JobRunnerIsValid(c.Runner) {
			return fmt.Errorf("invalid Runner value: parsing failed")
		}

		if err := (&RecordingJobInputData{}).FromMap(c.InputData).IsValid(); err != nil {
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
