// Copyright (c) 2022-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRetentionTimeParse(t *testing.T) {
	newRT := func(d time.Duration) *RetentionTime {
		rt := RetentionTime(d)
		return &rt
	}

	tcs := []struct {
		name     string
		data     []byte
		rt       *RetentionTime
		expected *RetentionTime
		err      string
	}{
		{
			name:     "nil pointer",
			data:     nil,
			rt:       nil,
			expected: nil,
			err:      "invalid nil pointer",
		},
		{
			name:     "invalid formatting",
			data:     []byte("10dd"),
			rt:       newRT(0),
			expected: newRT(0),
			err:      "invalid retention time format",
		},
		{
			name:     "mixed units",
			data:     []byte("10h10m"),
			rt:       newRT(0),
			expected: newRT(0),
			err:      "invalid retention time format",
		},
		{
			name:     "seconds",
			data:     []byte("45s"),
			rt:       newRT(0),
			expected: newRT(0),
			err:      "invalid retention time format",
		},
		{
			name:     "minutes",
			data:     []byte("45m"),
			rt:       newRT(0),
			expected: newRT(time.Minute * 45),
			err:      "",
		},
		{
			name:     "hours",
			data:     []byte("24h"),
			rt:       newRT(0),
			expected: newRT(time.Hour * 24),
			err:      "",
		},
		{
			name:     "days",
			data:     []byte("10d"),
			rt:       newRT(0),
			expected: newRT(time.Hour * 24 * 10),
			err:      "",
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.rt.UnmarshalText(tc.data)
			if tc.err != "" {
				require.EqualError(t, err, tc.err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.expected, tc.rt)
		})
	}
}
