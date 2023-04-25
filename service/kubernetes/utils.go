// Copyright (c) 2022-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package kubernetes

import (
	"fmt"
	"strings"

	recorder "github.com/mattermost/calls-recorder/cmd/recorder/config"

	corev1 "k8s.io/api/core/v1"
)

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
