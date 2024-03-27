// Copyright (c) 2022-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package job

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJobConfigIsValid(t *testing.T) {
	inputData := InputData{
		"site_url":     "http://localhost:8065",
		"call_id":      "8w8jorhr7j83uqr6y1st894hqe",
		"post_id":      "udzdsg7dwidbzcidx5khrf8nee",
		"auth_token":   "qj75unbsef83ik9p7ueypb6iyw",
		"recording_id": "dtomsek53i8eukrhnb31ugyhea",
	}

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
				InputData:      inputData,
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
				InputData: inputData,
			},
			registry:      ImageRegistryDefault,
			expectedError: fmt.Sprintf("invalid Runner value: actual version (0.1.0) is lower than minimum supported version (%s)", MinSupportedRecorderVersion),
		},
		{
			name: "invalid registry",
			cfg: Config{
				Type:           TypeRecording,
				Runner:         "mattermost/calls-recorder:v" + MinSupportedRecorderVersion,
				InputData:      inputData,
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
				InputData:      inputData,
				MaxDurationSec: 60,
			},
			registry: ImageRegistryDefault,
		},
		{
			name: "valid -dev build",
			cfg: Config{
				Type:           TypeRecording,
				Runner:         "mattermost/calls-recorder:v" + MinSupportedRecorderVersion + "-dev",
				InputData:      inputData,
				MaxDurationSec: 60,
			},
			registry: ImageRegistryDefault,
		},
		{
			name: "valid -dev# build",
			cfg: Config{
				Type:           TypeRecording,
				Runner:         "mattermost/calls-recorder:v" + MinSupportedRecorderVersion + "-dev29",
				InputData:      inputData,
				MaxDurationSec: 60,
			},
			registry: ImageRegistryDefault,
		},
		{
			name: "valid -dev build, transcriber",
			cfg: Config{
				Type:           TypeRecording,
				Runner:         "mattermost/calls-transcriber:v" + MinSupportedTranscriberVersion + "-dev",
				InputData:      inputData,
				MaxDurationSec: 60,
			},
			registry: ImageRegistryDefault,
		},
		{
			name: "valid -dev# build, transcriber",
			cfg: Config{
				Type:           TypeRecording,
				Runner:         "mattermost/calls-transcriber:v" + MinSupportedTranscriberVersion + "-dev341",
				InputData:      inputData,
				MaxDurationSec: 60,
			},
			registry: ImageRegistryDefault,
		},
		{
			name: "valid, non default registry",
			cfg: Config{
				Type:           TypeRecording,
				Runner:         "custom/calls-recorder:v" + MinSupportedRecorderVersion,
				InputData:      inputData,
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
		{
			name: "valid -dev config",
			cfg: ServiceConfig{
				Runners: []string{
					"mattermost/calls-recorder:v" + MinSupportedRecorderVersion + "-dev",
					"mattermost/calls-transcriber:v" + MinSupportedTranscriberVersion + "-dev",
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

func TestInputData(t *testing.T) {
	t.Run("GetSiteURL", func(t *testing.T) {
		t.Run("nil", func(t *testing.T) {
			var inputData InputData
			siteURL := inputData.GetSiteURL()
			require.Empty(t, siteURL)
		})

		t.Run("missing", func(t *testing.T) {
			inputData := InputData{}
			siteURL := inputData.GetSiteURL()
			require.Empty(t, siteURL)
		})

		t.Run("invalid type", func(t *testing.T) {
			inputData := InputData{
				"site_url": 45,
			}
			siteURL := inputData.GetSiteURL()
			require.Empty(t, siteURL)
		})

		t.Run("valid", func(t *testing.T) {
			inputData := InputData{
				"site_url": "http://localhost:8065",
			}
			siteURL := inputData.GetSiteURL()
			require.Equal(t, "http://localhost:8065", siteURL)
		})
	})

	t.Run("SetSiteURL", func(t *testing.T) {
		t.Run("nil", func(t *testing.T) {
			siteURL := "http://localhost:8065"
			var inputData InputData
			inputData.SetSiteURL(siteURL)
			url := inputData.GetSiteURL()
			require.Empty(t, url)
		})

		t.Run("set", func(t *testing.T) {
			siteURL := "http://localhost:8065"
			inputData := InputData{}
			inputData.SetSiteURL(siteURL)
			url := inputData.GetSiteURL()
			require.Equal(t, siteURL, url)
		})

		t.Run("update", func(t *testing.T) {
			siteURL := "http://localhost:8065"
			inputData := InputData{
				"site_url": 45,
			}
			inputData.SetSiteURL(siteURL)
			url := inputData.GetSiteURL()
			require.Equal(t, siteURL, url)
		})
	})

	t.Run("ToEnv", func(t *testing.T) {
		t.Run("nil", func(t *testing.T) {
			var inputData InputData
			env := inputData.ToEnv()
			require.Empty(t, env)
		})

		t.Run("set", func(t *testing.T) {
			inputData := InputData{
				"site_url":      "http://localhost:8065",
				"call_id":       "callID",
				"video_rate":    1000,
				"output_format": "format",
			}

			env := inputData.ToEnv()
			require.Equal(t, []string{
				"SITE_URL=http://localhost:8065",
				"CALL_ID=callID",
				"VIDEO_RATE=1000",
				"OUTPUT_FORMAT=format",
			}, env)
		})
	})
}
