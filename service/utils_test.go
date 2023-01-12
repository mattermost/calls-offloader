package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckMinVersion(t *testing.T) {
	tcs := []struct {
		name          string
		minVersion    string
		actualVersion string
		err           string
	}{
		{
			name:          "empty minVersion",
			minVersion:    "",
			actualVersion: "",
			err:           "failed to parse minVersion: Invalid Semantic Version",
		},
		{
			name:          "empty actualVersion",
			minVersion:    "0.1.0",
			actualVersion: "",
			err:           "failed to parse actualVersion: Invalid Semantic Version",
		},
		{
			name:          "invalid minVersion",
			minVersion:    "not.a.version",
			actualVersion: "not.a.version",
			err:           "failed to parse minVersion: Invalid Semantic Version",
		},
		{
			name:          "invalid actualVersion",
			minVersion:    "0.1.0",
			actualVersion: "not.a.version",
			err:           "failed to parse actualVersion: Invalid Semantic Version",
		},
		{
			name:          "not supported, minor",
			minVersion:    "0.2.0",
			actualVersion: "0.1.0",
			err:           "actual version (0.1.0) is lower than minimum supported version (0.2.0)",
		},
		{
			name:          "not supported, patch",
			minVersion:    "0.2.1",
			actualVersion: "0.2.0",
			err:           "actual version (0.2.0) is lower than minimum supported version (0.2.1)",
		},
		{
			name:          "supported, equal",
			minVersion:    "0.2.1",
			actualVersion: "0.2.1",
			err:           "",
		},
		{
			name:          "supported, greater",
			minVersion:    "0.2.1",
			actualVersion: "0.2.2",
			err:           "",
		},
		{
			name:          "supported, minVersion prefix",
			minVersion:    "v0.2.1",
			actualVersion: "0.2.2",
			err:           "",
		},
		{
			name:          "supported, actualVersion prefix",
			minVersion:    "0.2.1",
			actualVersion: "v0.2.2",
			err:           "",
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			err := checkMinVersion(tc.minVersion, tc.actualVersion)
			if tc.err != "" {
				assert.EqualError(t, err, tc.err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
