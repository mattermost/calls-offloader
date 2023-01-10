// Copyright (c) 2022-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRecordingJobInputDataIsValid(t *testing.T) {
	tcs := []struct {
		name          string
		data          RecordingJobInputData
		expectedError string
	}{
		{
			name:          "empty data",
			data:          RecordingJobInputData{},
			expectedError: "invalid SiteURL value: should not be empty",
		},
		{
			name: "invalid SiteURL schema",
			data: RecordingJobInputData{
				SiteURL: "invalid://localhost",
			},
			expectedError: "SiteURL parsing failed: invalid scheme \"invalid\"",
		},
		{
			name: "missing CallID",
			data: RecordingJobInputData{
				SiteURL: "http://localhost:8065",
			},
			expectedError: "invalid CallID value: should not be empty",
		},
		{
			name: "invalid CallID",
			data: RecordingJobInputData{
				SiteURL: "http://localhost:8065",
				CallID:  "invalid",
			},
			expectedError: "CallID parsing failed",
		},
		{
			name: "missing ThreadID",
			data: RecordingJobInputData{
				SiteURL:   "http://localhost:8065",
				CallID:    "8w8jorhr7j83uqr6y1st894hqe",
				AuthToken: "qj75unbsef83ik9p7ueypb6iyw",
			},
			expectedError: "invalid ThreadID value: should not be empty",
		},
		{
			name: "invalid ThreadID",
			data: RecordingJobInputData{
				SiteURL:  "http://localhost:8065",
				ThreadID: "invalid",
				CallID:   "8w8jorhr7j83uqr6y1st894hqe",
			},
			expectedError: "ThreadID parsing failed",
		},
		{
			name: "missing AuthToken",
			data: RecordingJobInputData{
				SiteURL:  "http://localhost:8065",
				CallID:   "8w8jorhr7j83uqr6y1st894hqe",
				ThreadID: "udzdsg7dwidbzcidx5khrf8nee",
			},
			expectedError: "invalid AuthToken value: should not be empty",
		},
		{
			name: "invalid AuthToken",
			data: RecordingJobInputData{
				SiteURL:   "http://localhost:8065",
				ThreadID:  "udzdsg7dwidbzcidx5khrf8nee",
				CallID:    "8w8jorhr7j83uqr6y1st894hqe",
				AuthToken: "invalid",
			},
			expectedError: "AuthToken parsing failed",
		},
		{
			name: "valid config",
			data: RecordingJobInputData{
				SiteURL:   "http://localhost:8065",
				CallID:    "8w8jorhr7j83uqr6y1st894hqe",
				ThreadID:  "udzdsg7dwidbzcidx5khrf8nee",
				AuthToken: "qj75unbsef83ik9p7ueypb6iyw",
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.data.IsValid()
			if tc.expectedError == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.expectedError)
			}
		})
	}
}
