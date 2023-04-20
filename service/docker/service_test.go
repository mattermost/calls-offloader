// Copyright (c) 2022-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package docker

import (
	"testing"

	"github.com/mattermost/mattermost-server/v6/shared/mlog"

	"github.com/stretchr/testify/require"
)

func TestNewJobService(t *testing.T) {
	log, err := mlog.NewLogger()
	require.NoError(t, err)
	defer func() {
		err := log.Shutdown()
		require.NoError(t, err)
	}()

	jobService, err := NewJobService(log, JobServiceConfig{})
	require.Nil(t, err)
	require.NotNil(t, jobService)

	err = jobService.Shutdown()
	require.Nil(t, err)
}
