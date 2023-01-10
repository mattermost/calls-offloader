// Copyright (c) 2022-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package service

import (
	"fmt"
	"strings"
)

type JobType string

const (
	JobTypeRecording     JobType = "recording"
	recorderRunnerPrefix         = "mattermost/calls-recorder@"
)

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

func (c JobConfig) IsValid() error {
	if c.Type == "" {
		return fmt.Errorf("invalid Type value: should not be empty")
	}

	if c.Runner == "" {
		return fmt.Errorf("invalid Runner value: should not be empty")
	}

	switch c.Type {
	case JobTypeRecording:
		if !strings.HasPrefix(c.Runner, recorderRunnerPrefix) {
			return fmt.Errorf("invalid Runner value: missing prefix")
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
