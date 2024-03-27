package kubernetes

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/mattermost/calls-offloader/public/job"

	corev1 "k8s.io/api/core/v1"

	"github.com/stretchr/testify/require"
)

func TestGetEnvFromJobInputData(t *testing.T) {
	tcs := []struct {
		name string
		data job.InputData
		env  []corev1.EnvVar
	}{
		{
			name: "empty data",
			data: job.InputData{},
			env:  []corev1.EnvVar(nil),
		},
		{
			name: "valid data",
			data: job.InputData{
				"site_url":      "http://localhost:8065",
				"auth_token":    "authToken",
				"call_id":       "callID",
				"post_id":       "postID",
				"recording_id":  "recordingID",
				"width":         1920,
				"height":        1080,
				"video_rate":    1500,
				"audio_rate":    64,
				"frame_rate":    30,
				"video_preset":  "fast",
				"output_format": "mp4",
			},
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
					Name:  "POST_ID",
					Value: "postID",
				},
				{
					Name:  "RECORDING_ID",
					Value: "recordingID",
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
			env := getEnvFromJobInputData(tc.data)
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
