// Copyright (c) 2022-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package service

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParseRetentionTime(t *testing.T) {
	tcs := []struct {
		name     string
		input    string
		expected time.Duration
		err      string
	}{
		{
			name:     "invalid formatting",
			input:    "10dd",
			expected: 0,
			err:      "invalid retention time format",
		},
		{
			name:     "mixed units",
			input:    "10h10m",
			expected: 0,
			err:      "invalid retention time format",
		},
		{
			name:     "seconds",
			input:    "45s",
			expected: 0,
			err:      "invalid retention time format",
		},
		{
			name:     "minutes",
			input:    "45m",
			expected: time.Minute * 45,
			err:      "",
		},
		{
			name:     "hours",
			input:    "24h",
			expected: time.Hour * 24,
			err:      "",
		},
		{
			name:     "days",
			input:    "10d",
			expected: time.Hour * 24 * 10,
			err:      "",
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			d, err := parseRetentionTime(tc.input)
			if tc.err != "" {
				require.EqualError(t, err, tc.err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.expected, d)
		})
	}
}

func TestParseFromEnv(t *testing.T) {
	t.Run("no env", func(t *testing.T) {
		var cfg Config
		err := cfg.ParseFromEnv()
		require.NoError(t, err)
		require.Empty(t, cfg)
	})

	t.Run("FailedJobsRetentionTime", func(t *testing.T) {
		os.Setenv("JOBS_FAILEDJOBSRETENTIONTIME", "1d")
		defer os.Unsetenv("JOBS_FAILEDJOBSRETENTIONTIME")

		var cfg Config
		err := cfg.ParseFromEnv()
		require.NoError(t, err)
		require.Equal(t, time.Hour*24, cfg.Jobs.FailedJobsRetentionTime)
	})

	t.Run("override", func(t *testing.T) {
		var cfg Config
		cfg.Jobs.APIType = JobAPITypeKubernetes

		os.Setenv("JOBS_APITYPE", "docker")
		defer os.Unsetenv("JOBS_APITYPE")
		err := cfg.ParseFromEnv()
		require.NoError(t, err)
		require.Equal(t, JobAPITypeDocker, cfg.Jobs.APIType)
	})
}
