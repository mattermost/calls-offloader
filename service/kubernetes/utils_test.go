package kubernetes

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/mattermost/calls-offloader/public/job"
	"github.com/mattermost/mattermost/server/public/shared/mlog"

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

func TestGenInitContainers(t *testing.T) {
	for _, tc := range []struct {
		name    string
		jobID   string
		image   string
		sysctls string
		err     string
		cnts    []corev1.Container
	}{
		{
			name: "empty jobID",
			err:  "invalid empty jobID",
		},
		{
			name:  "empty image",
			jobID: "jobID",
			err:   "invalid empty image",
		},
		{
			name:  "empty sysctls",
			jobID: "jobID",
			image: "image",
			err:   "invalid empty sysctls",
		},
		{
			name:    "single sysctl",
			jobID:   "jobID",
			image:   "image",
			sysctls: "kernel.unprivileged_userns_clone=1",
			cnts: []corev1.Container{
				{
					Name:            "jobID-init-0",
					Image:           "image",
					ImagePullPolicy: corev1.PullIfNotPresent,
					Command: []string{
						"sysctl",
						"-w",
						"kernel.unprivileged_userns_clone=1",
					},
					SecurityContext: &corev1.SecurityContext{
						Privileged: newBool(false),
					},
				},
			},
		},
		{
			name:    "multiple sysctls",
			jobID:   "jobID",
			image:   "image",
			sysctls: "kernel.unprivileged_userns_clone=1,user.max_user_namespaces=4545",
			cnts: []corev1.Container{
				{
					Name:            "jobID-init-0",
					Image:           "image",
					ImagePullPolicy: corev1.PullIfNotPresent,
					Command: []string{
						"sysctl",
						"-w",
						"kernel.unprivileged_userns_clone=1",
					},
					SecurityContext: &corev1.SecurityContext{
						Privileged: newBool(false),
					},
				},
				{
					Name:            "jobID-init-1",
					Image:           "image",
					ImagePullPolicy: corev1.PullIfNotPresent,
					Command: []string{
						"sysctl",
						"-w",
						"user.max_user_namespaces=4545",
					},
					SecurityContext: &corev1.SecurityContext{
						Privileged: newBool(false),
					},
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cnts, err := genInitContainers(tc.jobID, tc.image, tc.sysctls)
			if tc.err != "" {
				require.Empty(t, cnts)
				require.EqualError(t, err, tc.err)
			} else {
				require.Equal(t, tc.cnts, cnts)
			}
		})
	}
}

func TestGenInitContainersWithSecurityContext(t *testing.T) {
	t.Run("init containers use non-privileged by default", func(t *testing.T) {
		os.Unsetenv("SECURITY_CONTEXT_PRIVILEGED")

		cnts, err := genInitContainers("test-job", "test-image", "kernel.unprivileged_userns_clone=1")
		require.NoError(t, err)
		require.Len(t, cnts, 1)
		require.NotNil(t, cnts[0].SecurityContext)
		require.NotNil(t, cnts[0].SecurityContext.Privileged)
		require.False(t, *cnts[0].SecurityContext.Privileged)
	})

	t.Run("init containers use privileged when env var is true", func(t *testing.T) {
		os.Setenv("SECURITY_CONTEXT_PRIVILEGED", "true")
		defer os.Unsetenv("SECURITY_CONTEXT_PRIVILEGED")

		cnts, err := genInitContainers("test-job", "test-image", "kernel.unprivileged_userns_clone=1")
		require.NoError(t, err)
		require.Len(t, cnts, 1)
		require.NotNil(t, cnts[0].SecurityContext)
		require.NotNil(t, cnts[0].SecurityContext.Privileged)
		require.True(t, *cnts[0].SecurityContext.Privileged)
	})

	t.Run("init containers use non-privileged when env var is false", func(t *testing.T) {
		os.Setenv("SECURITY_CONTEXT_PRIVILEGED", "false")
		defer os.Unsetenv("SECURITY_CONTEXT_PRIVILEGED")

		cnts, err := genInitContainers("test-job", "test-image", "kernel.unprivileged_userns_clone=1")
		require.NoError(t, err)
		require.Len(t, cnts, 1)
		require.NotNil(t, cnts[0].SecurityContext)
		require.NotNil(t, cnts[0].SecurityContext.Privileged)
		require.False(t, *cnts[0].SecurityContext.Privileged)
	})

	t.Run("multiple init containers all use same security context", func(t *testing.T) {
		os.Setenv("SECURITY_CONTEXT_PRIVILEGED", "true")
		defer os.Unsetenv("SECURITY_CONTEXT_PRIVILEGED")

		cnts, err := genInitContainers("test-job", "test-image", "kernel.unprivileged_userns_clone=1,user.max_user_namespaces=4545")
		require.NoError(t, err)
		require.Len(t, cnts, 2)

		// Both containers should have privileged=true
		for i, cnt := range cnts {
			require.NotNil(t, cnt.SecurityContext, "Container %d should have SecurityContext", i)
			require.NotNil(t, cnt.SecurityContext.Privileged, "Container %d should have Privileged set", i)
			require.True(t, *cnt.SecurityContext.Privileged, "Container %d should be privileged", i)
		}
	})

	t.Run("init containers respect case insensitive env var", func(t *testing.T) {
		os.Setenv("SECURITY_CONTEXT_PRIVILEGED", "TRUE")
		defer os.Unsetenv("SECURITY_CONTEXT_PRIVILEGED")

		cnts, err := genInitContainers("test-job", "test-image", "kernel.unprivileged_userns_clone=1")
		require.NoError(t, err)
		require.Len(t, cnts, 1)
		require.NotNil(t, cnts[0].SecurityContext)
		require.NotNil(t, cnts[0].SecurityContext.Privileged)
		require.True(t, *cnts[0].SecurityContext.Privileged)
	})
}

func TestGetVolumesAndMounts(t *testing.T) {
	log, err := mlog.NewLogger()
	require.NoError(t, err)

	t.Run("default, no PVC, no cert", func(t *testing.T) {
		volumes, mounts, err := getVolumesAndMounts("job-1", "", log)
		require.NoError(t, err)
		require.Len(t, volumes, 1)
		require.Len(t, mounts, 1)

		require.Equal(t, "job-1", volumes[0].Name)
		require.Nil(t, volumes[0].VolumeSource.PersistentVolumeClaim)

		require.Equal(t, "job-1", mounts[0].Name)
		require.Equal(t, "/data", mounts[0].MountPath)
	})

	t.Run("with PVC", func(t *testing.T) {
		volumes, mounts, err := getVolumesAndMounts("job-2", "my-pvc", log)
		require.NoError(t, err)
		require.Len(t, volumes, 1)
		require.Len(t, mounts, 1)

		require.NotNil(t, volumes[0].VolumeSource.PersistentVolumeClaim)
		require.Equal(t, "my-pvc", volumes[0].VolumeSource.PersistentVolumeClaim.ClaimName)
	})

	t.Run("with ConfigMap cert", func(t *testing.T) {
		t.Setenv("JOBS_K8S_CERT_CONFIGMAP", "my-ca-configmap")

		volumes, mounts, err := getVolumesAndMounts("job-3", "", log)
		require.NoError(t, err)
		require.Len(t, volumes, 2)
		require.Len(t, mounts, 2)

		// Data volume
		require.Equal(t, "job-3", volumes[0].Name)

		// Cert volume
		require.Equal(t, "certs", volumes[1].Name)
		require.NotNil(t, volumes[1].VolumeSource.ConfigMap)
		require.Equal(t, "my-ca-configmap", volumes[1].VolumeSource.ConfigMap.Name)

		// Cert mount
		require.Equal(t, "certs", mounts[1].Name)
		require.Equal(t, "/certs", mounts[1].MountPath)
		require.True(t, mounts[1].ReadOnly)
	})

	t.Run("with Secret cert", func(t *testing.T) {
		t.Setenv("JOBS_K8S_CERT_SECRET", "my-ca-secret")

		volumes, mounts, err := getVolumesAndMounts("job-4", "", log)
		require.NoError(t, err)
		require.Len(t, volumes, 2)
		require.Len(t, mounts, 2)

		// Cert volume
		require.Equal(t, "certs", volumes[1].Name)
		require.NotNil(t, volumes[1].VolumeSource.Secret)
		require.Equal(t, "my-ca-secret", volumes[1].VolumeSource.Secret.SecretName)

		// Cert mount
		require.Equal(t, "certs", mounts[1].Name)
		require.Equal(t, "/certs", mounts[1].MountPath)
		require.True(t, mounts[1].ReadOnly)
	})

	t.Run("with PVC and ConfigMap cert", func(t *testing.T) {
		t.Setenv("JOBS_K8S_CERT_CONFIGMAP", "my-ca-configmap")

		volumes, mounts, err := getVolumesAndMounts("job-5", "my-pvc", log)
		require.NoError(t, err)
		require.Len(t, volumes, 2)
		require.Len(t, mounts, 2)

		// Data volume has PVC
		require.NotNil(t, volumes[0].VolumeSource.PersistentVolumeClaim)
		require.Equal(t, "my-pvc", volumes[0].VolumeSource.PersistentVolumeClaim.ClaimName)

		// Cert volume has ConfigMap
		require.Equal(t, "certs", volumes[1].Name)
		require.NotNil(t, volumes[1].VolumeSource.ConfigMap)
	})

	t.Run("both ConfigMap and Secret set", func(t *testing.T) {
		t.Setenv("JOBS_K8S_CERT_CONFIGMAP", "my-ca-configmap")
		t.Setenv("JOBS_K8S_CERT_SECRET", "my-ca-secret")

		volumes, mounts, err := getVolumesAndMounts("job-6", "", log)
		require.EqualError(t, err, "JOBS_K8S_CERT_CONFIGMAP and JOBS_K8S_CERT_SECRET are mutually exclusive, set only one")
		require.Nil(t, volumes)
		require.Nil(t, mounts)
	})
}
