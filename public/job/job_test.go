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
	recorderCfg.PostID = "udzdsg7dwidbzcidx5khrf8nee"
	recorderCfg.AuthToken = "qj75unbsef83ik9p7ueypb6iyw"
	recorderCfg.RecordingID = "dtomsek53i8eukrhnb31ugyhea"

	tcs := []struct {
		name          string
		cfg           Config
		registry      string
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
			registry:      ImageRegistryDefault,
			expectedError: "invalid Runner value: runner should not be empty",
		},
		{
			name: "empty registry",
			cfg: Config{
				Type:   TypeRecording,
				Runner: "testrepo/calls-recorder:v0.1.0",
			},
			expectedError: "invalid Runner value: registry should not be empty",
		},
		{
			name: "invalid runner",
			cfg: Config{
				Type:   TypeRecording,
				Runner: "testrepo/calls-recorder:v0.1.0",
			},
			registry:      ImageRegistryDefault,
			expectedError: `invalid Runner value: failed to validate runner "testrepo/calls-recorder:v0.1.0"`,
		},
		{
			name: "invalid runner",
			cfg: Config{
				Type:   TypeRecording,
				Runner: "testrepo/calls-recorder@sha256:abcde",
			},
			registry:      ImageRegistryDefault,
			expectedError: `invalid Runner value: failed to validate runner "testrepo/calls-recorder@sha256:abcde"`,
		},
		{
			name: "invalid max duration",
			cfg: Config{
				Type:           TypeRecording,
				Runner:         "mattermost/calls-recorder:v" + MinSupportedRecorderVersion,
				InputData:      recorderCfg.ToMap(),
				MaxDurationSec: -1,
			},
			registry:      ImageRegistryDefault,
			expectedError: "invalid MaxDurationSec value: should be positive",
		},
		{
			name: "invalid job type",
			cfg: Config{
				Type:           "invalid",
				Runner:         "mattermost/calls-recorder:v" + MinSupportedRecorderVersion,
				MaxDurationSec: 60,
			},
			registry:      ImageRegistryDefault,
			expectedError: "invalid Type value: \"invalid\"",
		},
		{
			name: "invalid version",
			cfg: Config{
				Type:      TypeRecording,
				Runner:    "mattermost/calls-recorder:v0.1.0",
				InputData: recorderCfg.ToMap(),
			},
			registry:      ImageRegistryDefault,
			expectedError: fmt.Sprintf("invalid Runner value: actual version (0.1.0) is lower than minimum supported version (%s)", MinSupportedRecorderVersion),
		},
		{
			name: "invalid registry",
			cfg: Config{
				Type:           TypeRecording,
				Runner:         "mattermost/calls-recorder:v" + MinSupportedRecorderVersion,
				InputData:      recorderCfg.ToMap(),
				MaxDurationSec: 60,
			},
			registry:      "custom",
			expectedError: fmt.Sprintf("invalid Runner value: failed to validate runner \"mattermost/calls-recorder:v%s\"", MinSupportedRecorderVersion),
		},
		{
			name: "valid",
			cfg: Config{
				Type:           TypeRecording,
				Runner:         "mattermost/calls-recorder:v" + MinSupportedRecorderVersion,
				InputData:      recorderCfg.ToMap(),
				MaxDurationSec: 60,
			},
			registry: ImageRegistryDefault,
		},
		{
			name: "valid daily",
			cfg: Config{
				Type:           TypeRecording,
				Runner:         "mattermost/calls-recorder-daily:v" + MinSupportedRecorderVersion + "-dev",
				InputData:      recorderCfg.ToMap(),
				MaxDurationSec: 60,
			},
			registry: ImageRegistryDefault,
		},
		{
			name: "valid, non default registry",
			cfg: Config{
				Type:           TypeRecording,
				Runner:         "custom/calls-recorder:v" + MinSupportedRecorderVersion,
				InputData:      recorderCfg.ToMap(),
				MaxDurationSec: 60,
			},
			registry: "custom",
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.IsValid(tc.registry)
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
			err:  "invalid empty Runners",
		},
		{
			name: "valid config",
			cfg: ServiceConfig{
				Runners: []string{
					"mattermost/calls-recorder:v" + MinSupportedRecorderVersion,
					"mattermost/calls-transcriber:v" + MinSupportedTranscriberVersion,
				},
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.IsValid(ImageRegistryDefault)
			if tc.err == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.err)
			}
		})
	}
}
