// Copyright (c) 2022-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package docker

import (
	"bytes"
	"io"
	"testing"
	"time"

	"github.com/mattermost/calls-offloader/service/job"

	"github.com/mattermost/mattermost-server/v6/shared/mlog"

	"github.com/stretchr/testify/require"
)

const testRunner = "hello-world"

func setupJobService(t *testing.T) (*JobService, func()) {
	t.Helper()

	log, err := mlog.NewLogger()
	require.NoError(t, err)

	jobService, err := NewJobService(log, JobServiceConfig{MaxConcurrentJobs: 100})
	require.NoError(t, err)
	require.NotNil(t, jobService)

	teardownFn := func() {
		err := jobService.Shutdown()
		require.NoError(t, err)

		err = log.Shutdown()
		require.NoError(t, err)
	}

	return jobService, teardownFn
}

func TestNewJobService(t *testing.T) {
	log, err := mlog.NewLogger()
	require.NoError(t, err)
	defer func() {
		err := log.Shutdown()
		require.NoError(t, err)
	}()

	jobService, err := NewJobService(log, JobServiceConfig{})
	require.NoError(t, err)
	require.NotNil(t, jobService)

	err = jobService.Shutdown()
	require.NoError(t, err)
}

func TestUpdateJobRunner(t *testing.T) {
	jobService, teardown := setupJobService(t)
	defer teardown()

	err := jobService.UpdateJobRunner(testRunner)
	require.NoError(t, err)
}

func TestCreateJob(t *testing.T) {
	jobService, teardown := setupJobService(t)
	defer teardown()

	stopCh := make(chan struct{})
	job, err := jobService.CreateJob(job.Config{
		Type:   job.TypeRecording,
		Runner: testRunner,
	}, func(_ job.Job, exitCode int) error {
		require.Zero(t, exitCode)
		close(stopCh)
		return nil
	})
	require.NoError(t, err)
	require.NotEmpty(t, job.ID)

	err = jobService.StopJob(job.ID)
	require.NoError(t, err)

	select {
	case <-stopCh:
	case <-time.After(2 * time.Second):
		require.FailNow(t, "timed out waiting for stopCh")
	}

	var buf bytes.Buffer
	err = jobService.GetJobLogs(job.ID, &buf, io.Discard)
	require.NoError(t, err)
	require.Contains(t, buf.String(), "Hello from Docker!")

	err = jobService.DeleteJob(job.ID)
	require.NoError(t, err)
}
