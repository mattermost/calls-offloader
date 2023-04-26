package kubernetes

import (
	"encoding/json"
	"os"
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

func TestGetJobPodTolerations(t *testing.T) {
	expectedTolerations := []corev1.Toleration{
		{
			Key:      "test1",
			Operator: corev1.TolerationOpEqual,
			Value:    "true",
			Effect:   corev1.TaintEffectNoSchedule,
		},
		{
			Key:      "test2",
			Operator: corev1.TolerationOpEqual,
			Value:    "true",
			Effect:   corev1.TaintEffectNoSchedule,
		},
	}

	t.Run("use defaults if no given", func(t *testing.T) {
		tolerations, err := getJobPodTolerations()
		require.NoError(t, err)
		require.Equal(t, defaultTolerations, tolerations)
	})

	t.Run("use given tolerations", func(t *testing.T) {
		data, err := json.Marshal(expectedTolerations)
		require.NoError(t, err)

		os.Setenv("K8S_JOB_POD_TOLERATIONS", string(data))
		defer os.Unsetenv("K8S_JOB_POD_TOLERATIONS")

		tolerations, err := getJobPodTolerations()
		require.NoError(t, err)
		require.NotEqual(t, defaultTolerations, tolerations)
		require.Equal(t, expectedTolerations, tolerations)
	})

	t.Run("invalid tolerations data", func(t *testing.T) {
		os.Setenv("K8S_JOB_POD_TOLERATIONS", "invalid data")
		defer os.Unsetenv("K8S_JOB_POD_TOLERATIONS")

		tolerations, err := getJobPodTolerations()
		require.Empty(t, tolerations)
		require.EqualError(t, err, "failed to unmarshal tolerations: error unmarshaling JSON: while decoding JSON: json: cannot unmarshal string into Go value of type []v1.Toleration")
	})

	t.Run("yaml file", func(t *testing.T) {
		os.Setenv("K8S_JOB_POD_TOLERATIONS_FILE", "../../testfiles/tolerations.yaml")
		defer os.Unsetenv("K8S_JOB_POD_TOLERATIONS_FILE")

		tolerations, err := getJobPodTolerations()
		require.NoError(t, err)
		require.Equal(t, expectedTolerations, tolerations)
	})

	t.Run("json file", func(t *testing.T) {
		os.Setenv("K8S_JOB_POD_TOLERATIONS_FILE", "../../testfiles/tolerations.json")
		defer os.Unsetenv("K8S_JOB_POD_TOLERATIONS_FILE")

		tolerations, err := getJobPodTolerations()
		require.NoError(t, err)
		require.Equal(t, expectedTolerations, tolerations)
	})

	t.Run("invalid file", func(t *testing.T) {
		os.Setenv("K8S_JOB_POD_TOLERATIONS_FILE", "invalid")
		defer os.Unsetenv("K8S_JOB_POD_TOLERATIONS_FILE")

		tolerations, err := getJobPodTolerations()
		require.Empty(t, tolerations)
		require.EqualError(t, err, "failed to open file invalid: open invalid: no such file or directory")
	})
}
