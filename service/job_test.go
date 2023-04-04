// Copyright (c) 2022-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package service

import (
	"testing"

	recorder "github.com/mattermost/calls-recorder/cmd/recorder/config"

	"github.com/stretchr/testify/require"
)

func TestJobConfigIsValid(t *testing.T) {
	var recorderCfg recorder.RecorderConfig
	recorderCfg.SetDefaults()
	recorderCfg.SiteURL = "http://localhost:8065"
	recorderCfg.CallID = "8w8jorhr7j83uqr6y1st894hqe"
	recorderCfg.ThreadID = "udzdsg7dwidbzcidx5khrf8nee"
	recorderCfg.AuthToken = "qj75unbsef83ik9p7ueypb6iyw"

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
				Type:           JobTypeRecording,
				Runner:         "mattermost/calls-recorder:v0.3.1",
				InputData:      recorderCfg.ToMap(),
				MaxDurationSec: -1,
			},
			expectedError: "invalid MaxDurationSec value: should not be negative",
		},
		{
			name: "invalid version",
			cfg: JobConfig{
				Type:      JobTypeRecording,
				Runner:    "mattermost/calls-recorder:v0.1.0",
				InputData: recorderCfg.ToMap(),
			},
			expectedError: "invalid Runner value: actual version (0.1.0) is lower than minimum supported version (0.3.1)",
		},
		{
			name: "valid",
			cfg: JobConfig{
				Type:           JobTypeRecording,
				Runner:         "mattermost/calls-recorder:v0.3.1",
				InputData:      recorderCfg.ToMap(),
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
