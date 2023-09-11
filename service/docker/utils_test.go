// Copyright (c) 2022-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package docker

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetImageNameFromRunner(t *testing.T) {
	tcs := []struct {
		name     string
		runner   string
		expected string
	}{
		{
			name:     "empty string",
			runner:   "",
			expected: "",
		},
		{
			name:     "invalid",
			runner:   "mattermost/invalid",
			expected: "",
		},
		{
			name:     "valid recorder",
			runner:   "mattermost/calls-recorder:v0.4.0",
			expected: "calls-recorder",
		},
		{
			name:     "valid transcriber",
			runner:   "mattermost/calls-transcriber:v0.1.0",
			expected: "calls-transcriber",
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected, getImageNameFromRunner(tc.runner))
		})
	}
}
