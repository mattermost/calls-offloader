// Copyright (c) 2022-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package docker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/mattermost/calls-offloader/public/job"

	recorder "github.com/mattermost/calls-recorder/cmd/recorder/config"

	"github.com/mattermost/mattermost/server/public/shared/mlog"

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

func TestInit(t *testing.T) {
	jobService, teardown := setupJobService(t)
	defer teardown()

	err := jobService.Init(job.ServiceConfig{Runners: []string{testRunner}})
	require.NoError(t, err)
}

func TestCreateJob(t *testing.T) {
	jobService, teardown := setupJobService(t)
	defer teardown()

	os.Setenv("TEST_MODE", "true")
	defer os.Unsetenv("TEST_MODE")

	var recCfg recorder.RecorderConfig
	recCfg.SetDefaults()
	recCfg.SiteURL = "http://localhost:8065"
	recCfg.CallID = "8w8jorhr7j83uqr6y1st894hqe"
	recCfg.ThreadID = "udzdsg7dwidbzcidx5khrf8nee"
	recCfg.AuthToken = "qj75unbsef83ik9p7ueypb6iyw"
	recCfg.RecordingID = "dtomsek53i8eukrhnb31ugyhea"

	stopCh := make(chan struct{})
	job, err := jobService.CreateJob(job.Config{
		Type:           job.TypeRecording,
		Runner:         testRunner,
		MaxDurationSec: 60,
		InputData:      recCfg.ToMap(),
	}, func(_ job.Job, success bool) error {
		require.True(t, success)
		close(stopCh)
		return nil
	})
	require.NoError(t, err)
	require.NotEmpty(t, job.ID)

	err = jobService.stopJob(job.ID)
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

func TestFailedJobsRetention(t *testing.T) {
	log, err := mlog.NewLogger()
	require.NoError(t, err)

	interval := dockerRetentionJobInterval
	dockerRetentionJobInterval = time.Second
	defer func() {
		dockerRetentionJobInterval = interval
	}()

	jobService, err := NewJobService(log, JobServiceConfig{
		MaxConcurrentJobs:       100,
		FailedJobsRetentionTime: 5 * time.Second,
	})
	require.NoError(t, err)
	require.NotNil(t, jobService)

	stopCh := make(chan struct{})
	job, err := jobService.CreateJob(job.Config{
		Type:   job.TypeRecording,
		Runner: testRunner,
	}, func(_ job.Job, success bool) error {
		require.True(t, success)
		close(stopCh)
		return nil
	})
	require.NoError(t, err)
	require.NotEmpty(t, job.ID)

	err = jobService.stopJob(job.ID)
	require.NoError(t, err)

	select {
	case <-stopCh:
	case <-time.After(2 * time.Second):
		require.FailNow(t, "timed out waiting for stopCh")
	}

	// Verify the container still exists.
	ctx, cancel := context.WithTimeout(context.Background(), dockerRequestTimeout)
	defer cancel()
	_, err = jobService.client.ContainerInspect(ctx, job.ID)
	require.NoError(t, err)

	// Wait enough for the retention job to trigger.
	time.Sleep(8 * time.Second)

	// Verify the container has been deleted
	ctx, cancel = context.WithTimeout(context.Background(), dockerRequestTimeout)
	defer cancel()
	_, err = jobService.client.ContainerInspect(ctx, job.ID)
	require.EqualError(t, err, fmt.Sprintf("Error: No such container: %s", job.ID))

	err = jobService.Shutdown()
	require.NoError(t, err)

	err = log.Shutdown()
	require.NoError(t, err)
}
