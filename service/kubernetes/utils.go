// Copyright (c) 2022-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package kubernetes

import (
	"bytes"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/mattermost/calls-offloader/public/job"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
)

var defaultTolerations = []corev1.Toleration{
	{
		Key:      "utilities",
		Operator: corev1.TolerationOpEqual,
		Value:    "true",
		Effect:   corev1.TaintEffectNoSchedule,
	},
}

func newInt32(val int32) *int32 {
	p := new(int32)
	*p = val
	return p
}

func newInt64(val int64) *int64 {
	p := new(int64)
	*p = val
	return p
}

func newBool(val bool) *bool {
	p := new(bool)
	*p = val
	return p
}

func getEnvFromJobInputData(data job.InputData) []corev1.EnvVar {
	var env []corev1.EnvVar
	for k, v := range data {
		env = append(env, corev1.EnvVar{
			Name:  strings.ToUpper(k),
			Value: fmt.Sprintf("%v", v),
		})
	}
	return env
}

func getJobPodTolerations() ([]corev1.Toleration, error) {
	var tolerations []corev1.Toleration

	// We support two ways to configre custom tolerations for job's pods.
	// - K8S_JOB_POD_TOLERATIONS environment variable should contain the
	//   serialized list of tolerations in JSON format.
	// - K8S_JOB_POD_TOLERATIONS_FILE environment variable should point to
	//   a file that containes the list of tolerations. File content can be
	//   either in YAML or JSON format.

	if data := os.Getenv("K8S_JOB_POD_TOLERATIONS"); data != "" {
		if err := yaml.NewYAMLOrJSONDecoder(bytes.NewBuffer([]byte(data)), 0).Decode(&tolerations); err != nil {
			return nil, fmt.Errorf("failed to unmarshal tolerations: %w", err)
		}

		return tolerations, nil
	}

	if path := os.Getenv("K8S_JOB_POD_TOLERATIONS_FILE"); path != "" {
		file, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("failed to open file %s: %w", path, err)
		}
		defer file.Close()

		if err := yaml.NewYAMLOrJSONDecoder(file, 1).Decode(&tolerations); err != nil {
			return nil, fmt.Errorf("failed to unmarshal tolerations: %w", err)
		}

		return tolerations, nil
	}

	return defaultTolerations, nil
}

func getActiveJobs(jobs []batchv1.Job) int {
	var activeJobs int
	for _, jb := range jobs {
		if jb.Status.Failed > 0 || jb.Status.Succeeded > 0 {
			continue
		}
		activeJobs++
	}
	return activeJobs
}

func getSiteURLForJob(siteURL string) string {
	if os.Getenv("DEV_MODE") != "true" {
		return siteURL
	}

	u, err := url.Parse(siteURL)
	if err == nil && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1") {
		u.Host = "host.minikube.internal" + ":" + u.Port()
		siteURL = u.String()
	}

	return siteURL
}

func genInitContainers(jobID, image, sysctls string) ([]corev1.Container, error) {
	if jobID == "" {
		return nil, fmt.Errorf("invalid empty jobID")
	}

	if image == "" {
		return nil, fmt.Errorf("invalid empty image")
	}

	if sysctls == "" {
		return nil, fmt.Errorf("invalid empty sysctls")
	}

	ctls := strings.Split(sysctls, ",")
	cnts := make([]corev1.Container, len(ctls))
	for i, ctl := range ctls {
		cnts[i] = corev1.Container{
			Name:            fmt.Sprintf("%s-init-%d", jobID, i),
			Image:           image,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Command: []string{
				"sysctl",
				"-w",
				ctl,
			},
			SecurityContext: &corev1.SecurityContext{
				Privileged: newBool(true),
			},
		}
	}

	return cnts, nil
}
