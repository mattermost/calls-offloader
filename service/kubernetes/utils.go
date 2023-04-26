// Copyright (c) 2022-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package kubernetes

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	recorder "github.com/mattermost/calls-recorder/cmd/recorder/config"

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

func getEnvFromConfig(cfg recorder.RecorderConfig) []corev1.EnvVar {
	if cfg == (recorder.RecorderConfig{}) {
		return nil
	}

	var env []corev1.EnvVar
	for k, v := range cfg.ToMap() {
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
