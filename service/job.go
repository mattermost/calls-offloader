// Copyright (c) 2022-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package service

import (
	"fmt"
)

type JobType string

const (
	JobTypeRecording JobType = "recording"
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
	if c.Type != JobTypeRecording {
		return fmt.Errorf("invalid Type value: %s", c.Type)
	}

	if c.Runner == "" {
		return fmt.Errorf("invalid Runner value: should not be empty")
	}

	if c.MaxDurationSec < 0 {
		return fmt.Errorf("invalid MaxDurationSec value: should not be negative")
	}

	switch c.Type {
	case JobTypeRecording:
		if err := (&RecordingJobInputData{}).FromMap(c.InputData).IsValid(); err != nil {
			return fmt.Errorf("failed to validate InputData: %w", err)
		}
	}

	return nil
}
