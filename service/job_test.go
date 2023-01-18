// Copyright (c) 2022-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJobConfigIsValid(t *testing.T) {
	tcs := []struct {
		name          string
		cfg           JobConfig
		expectedError string
	}{
		{
			name:          "empty config",
			cfg:           JobConfig{},
			expectedError: "invalid Type value: should not be empty",
		},
		{
			name: "empty runner",
			cfg: JobConfig{
				Type: JobTypeRecording,
			},
			expectedError: "invalid Runner value: should not be empty",
		},
		{
			name: "invalid runner",
			cfg: JobConfig{
				Type:   JobTypeRecording,
				Runner: "testrepo/calls-recorder:v0.1.0",
			},
			expectedError: "invalid Runner value: failed to validate runner",
		},
		{
			name: "invalid runner",
			cfg: JobConfig{
				Type:   JobTypeRecording,
				Runner: "testrepo/calls-recorder@sha256:abcde",
			},
			expectedError: "invalid Runner value: failed to validate runner",
		},
		{
			name: "invalid job type",
			cfg: JobConfig{
				Type:   "invalid",
				Runner: "mattermost/calls-recorder:v0.1.0",
			},
			expectedError: "invalid Type value: \"invalid\"",
		},
		{
			name: "invalid max duration",
			cfg: JobConfig{
				Type:   JobTypeRecording,
				Runner: "mattermost/calls-recorder:v0.2.2",
				InputData: map[string]any{
					"site_url":   "http://localhost:8065",
					"call_id":    "8w8jorhr7j83uqr6y1st894hqe",
					"thread_id":  "udzdsg7dwidbzcidx5khrf8nee",
					"auth_token": "qj75unbsef83ik9p7ueypb6iyw",
				},
				MaxDurationSec: -1,
			},
			expectedError: "invalid MaxDurationSec value: should not be negative",
		},
		{
			name: "invalid version",
			cfg: JobConfig{
				Type:   JobTypeRecording,
				Runner: "mattermost/calls-recorder:v0.1.0",
				InputData: map[string]any{
					"site_url":   "http://localhost:8065",
					"call_id":    "8w8jorhr7j83uqr6y1st894hqe",
					"thread_id":  "udzdsg7dwidbzcidx5khrf8nee",
					"auth_token": "qj75unbsef83ik9p7ueypb6iyw",
				},
			},
			expectedError: "invalid Runner value: actual version (0.1.0) is lower than minimum supported version (0.2.2)",
		},
		{
			name: "valid",
			cfg: JobConfig{
				Type:   JobTypeRecording,
				Runner: "mattermost/calls-recorder:v0.2.2",
				InputData: map[string]any{
					"site_url":   "http://localhost:8065",
					"call_id":    "8w8jorhr7j83uqr6y1st894hqe",
					"thread_id":  "udzdsg7dwidbzcidx5khrf8nee",
					"auth_token": "qj75unbsef83ik9p7ueypb6iyw",
				},
				MaxDurationSec: 60,
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.IsValid()
			if tc.expectedError == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.expectedError)
			}
		})
	}
}