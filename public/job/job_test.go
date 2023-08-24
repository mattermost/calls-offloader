// Copyright (c) 2022-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package job

import (
	"fmt"
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
		cfg           Config
		expectedError string
	}{
		{
			name:          "empty config",
			cfg:           Config{},
			expectedError: "invalid Type value: should not be empty",
		},
		{
			name: "empty runner",
			cfg: Config{
				Type: TypeRecording,
			},
			expectedError: "invalid Runner value: should not be empty",
		},
		{
			name: "invalid runner",
			cfg: Config{
				Type:   TypeRecording,
				Runner: "testrepo/calls-recorder:v0.1.0",
			},
			expectedError: "invalid Runner value: failed to validate runner",
		},
		{
			name: "invalid runner",
			cfg: Config{
				Type:   TypeRecording,
				Runner: "testrepo/calls-recorder@sha256:abcde",
			},
			expectedError: "invalid Runner value: failed to validate runner",
		},
		{
			name: "invalid job type",
			cfg: Config{
				Type:   "invalid",
				Runner: "mattermost/calls-recorder:v0.1.0",
			},
			expectedError: "invalid Type value: \"invalid\"",
		},
		{
			name: "invalid max duration",
			cfg: Config{
				Type:           TypeRecording,
				Runner:         "mattermost/calls-recorder:v" + MinSupportedRecorderVersion,
				InputData:      recorderCfg.ToMap(),
				MaxDurationSec: -1,
			},
			expectedError: "invalid MaxDurationSec value: should be positive",
		},
		{
			name: "invalid version",
			cfg: Config{
				Type:      TypeRecording,
				Runner:    "mattermost/calls-recorder:v0.1.0",
				InputData: recorderCfg.ToMap(),
			},
			expectedError: fmt.Sprintf("invalid Runner value: actual version (0.1.0) is lower than minimum supported version (%s)", MinSupportedRecorderVersion),
		},
		{
			name: "valid",
			cfg: Config{
				Type:           TypeRecording,
				Runner:         "mattermost/calls-recorder:v" + MinSupportedRecorderVersion,
				InputData:      recorderCfg.ToMap(),
				MaxDurationSec: 60,
			},
		},
		{
			name: "valid daily",
			cfg: Config{
				Type:           TypeRecording,
				Runner:         "mattermost/calls-recorder-daily:v" + MinSupportedRecorderVersion + "-dev",
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

func TestServiceConfigIsValid(t *testing.T) {
	tcs := []struct {
		name string
		cfg  ServiceConfig
		err  string
	}{
		{
			name: "empty config",
			cfg:  ServiceConfig{},
			err:  "failed to validate runner",
		},
		{
			name: "valid config",
			cfg: ServiceConfig{
				Runner: "mattermost/calls-recorder:v" + MinSupportedRecorderVersion,
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.IsValid()
			if tc.err == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.err)
			}
		})
	}
}
