package kubernetes

import (
	"testing"

	recorder "github.com/mattermost/calls-recorder/cmd/recorder/config"

	corev1 "k8s.io/api/core/v1"

	"github.com/stretchr/testify/require"
)

func TestGetEnvFromConfig(t *testing.T) {
	tcs := []struct {
		name string
		cfg  recorder.RecorderConfig
		env  []corev1.EnvVar
	}{
		{
			name: "empty config",
			cfg:  recorder.RecorderConfig{},
			env:  []corev1.EnvVar(nil),
		},
		{
			name: "valid config",
			cfg: func() recorder.RecorderConfig {
				var cfg recorder.RecorderConfig
				cfg.SetDefaults()

				cfg.SiteURL = "http://localhost:8065"
				cfg.AuthToken = "authToken"
				cfg.CallID = "callID"
				cfg.ThreadID = "threadID"

				return cfg
			}(),
			env: []corev1.EnvVar{
				{
					Name:  "SITE_URL",
					Value: "http://localhost:8065",
				},
				{
					Name:  "AUTH_TOKEN",
					Value: "authToken",
				},
				{
					Name:  "CALL_ID",
					Value: "callID",
				},
				{
					Name:  "THREAD_ID",
					Value: "threadID",
				},
				{
					Name:  "WIDTH",
					Value: "1920",
				},
				{
					Name:  "HEIGHT",
					Value: "1080",
				},
				{
					Name:  "VIDEO_RATE",
					Value: "1500",
				},
				{
					Name:  "AUDIO_RATE",
					Value: "64",
				},
				{
					Name:  "FRAME_RATE",
					Value: "30",
				},
				{
					Name:  "VIDEO_PRESET",
					Value: "fast",
				},
				{
					Name:  "OUTPUT_FORMAT",
					Value: "mp4",
				},
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			env := getEnvFromConfig(tc.cfg)
			require.ElementsMatch(t, tc.env, env)
		})
	}
}
