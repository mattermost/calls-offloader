// Copyright (c) 2022-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/mattermost/mattermost-server/v6/shared/mlog"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	docker "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

const dockerRequestTimeout = 5 * time.Second

var dockerStopTimeout = 5 * time.Minute

type JobService struct {
	log *mlog.Logger
	cfg JobsConfig
}

func NewJobService(cfg JobsConfig, log *mlog.Logger) (*JobService, error) {
	if err := cfg.IsValid(); err != nil {
		return nil, fmt.Errorf("failed to validate config: %w", err)
	}

	return &JobService{
		log: log,
		cfg: cfg,
	}, nil
}

func (s *JobService) CreateRecordingJobDocker(cfg JobConfig, onStopCb func(job Job) error) (Job, error) {
	if onStopCb == nil {
		return Job{}, fmt.Errorf("onStopCb should not be nil")
	}

	job := Job{
		JobConfig: cfg,
	}

	ctx, cancel := context.WithTimeout(context.Background(), dockerRequestTimeout)
	defer cancel()
	cli, err := docker.NewClientWithOpts(docker.FromEnv, docker.WithAPIVersionNegotiation())
	if err != nil {
		return Job{}, fmt.Errorf("failed to create docker client: %w", err)
	}
	defer cli.Close()

	cnts, err := cli.ContainerList(ctx, types.ContainerListOptions{})
	if err != nil {
		return Job{}, fmt.Errorf("failed to list containers: %w", err)
	}

	if len(cnts) >= s.cfg.MaxConcurrentJobs {
		return Job{}, fmt.Errorf("max concurrent jobs reached")
	}

	if _, _, err := cli.ImageInspectWithRaw(ctx, job.Runner); err != nil {
		out, err := cli.ImagePull(ctx, job.Runner, types.ImagePullOptions{})
		if err != nil {
			return Job{}, fmt.Errorf("failed to pull docker image: %w", err)
		}
		defer out.Close()
		_, _ = io.Copy(io.Discard, out)
	}

	var jobData RecordingJobInputData
	jobData.FromMap(cfg.InputData)

	env := []string{
		fmt.Sprintf("SITE_URL=%s", jobData.SiteURL),
		fmt.Sprintf("CALL_ID=%s", jobData.CallID),
		fmt.Sprintf("THREAD_ID=%s", jobData.ThreadID),
		fmt.Sprintf("AUTH_TOKEN=%s", jobData.AuthToken),
	}
	if devMode := os.Getenv("DEV_MODE"); devMode == "true" {
		env = append(env, "DEV_MODE=true")
	}

	// TODO: review volume naming and cleanup.
	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image:   job.Runner,
		Tty:     false,
		Env:     env,
		Volumes: map[string]struct{}{"calls-recorder-volume:/recs": {}},
	}, &container.HostConfig{
		Mounts: []mount.Mount{
			{
				Target: "/recs",
				Source: "calls-recorder-volume",
				Type:   "volume",
			},
		},
	}, nil, nil, "")
	if err != nil {
		return Job{}, fmt.Errorf("failed to create container: %w", err)
	}

	job.ID = resp.ID

	if err := cli.ContainerStart(ctx, job.ID, types.ContainerStartOptions{}); err != nil {
		return Job{}, fmt.Errorf("failed to start container: %w", err)
	}

	job.StartAt = time.Now().UnixMilli()

	go func() {
		timeout := dockerRequestTimeout
		if cfg.MaxDurationSec > 0 {
			timeout = time.Duration(cfg.MaxDurationSec) * time.Second
		}

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		waitCh, errCh := cli.ContainerWait(ctx, job.ID, container.WaitConditionNotRunning)

		select {
		case <-waitCh:
			s.log.Debug("container exited", mlog.String("jobID", job.ID))
		case err := <-errCh:
			s.log.Warn("timeout reached, stopping job", mlog.Err(err), mlog.String("jobID", job.ID))
			if err := s.StopRecordingJobDocker(job.ID); err != nil {
				s.log.Error("failed to stop job", mlog.Err(err), mlog.String("jobID", job.ID))
				return
			}
		}

		if err := onStopCb(job); err != nil {
			s.log.Error("failed to run onStopCb", mlog.Err(err), mlog.String("jobID", job.ID))
		}
	}()

	return job, nil
}

func (s *JobService) StopRecordingJobDocker(jobID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), dockerRequestTimeout)
	defer cancel()
	cli, err := docker.NewClientWithOpts(docker.FromEnv, docker.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("failed to create docker client: %w", err)
	}
	defer cli.Close()

	if err := cli.ContainerStop(ctx, jobID, &dockerStopTimeout); err != nil {
		return fmt.Errorf("failed to stop container: %s", err.Error())
	}

	return nil
}

func (s *JobService) RecordingJobLogsDocker(jobID string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dockerRequestTimeout)
	defer cancel()
	cli, err := docker.NewClientWithOpts(docker.FromEnv, docker.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}
	defer cli.Close()

	rdr, err := cli.ContainerLogs(ctx, jobID, types.ContainerLogsOptions{
		ShowStderr: true,
		Since:      time.Now().Add(-time.Hour).Format(time.RFC3339),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get container logs: %s", err.Error())
	}

	defer rdr.Close()

	var buf bytes.Buffer
	_, err = stdcopy.StdCopy(io.Discard, &buf, rdr)
	if err != nil {
		return nil, fmt.Errorf("failed to read logs: %s", err.Error())
	}

	return buf.Bytes(), nil
}
