// Copyright (c) 2022-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package service

import (
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
