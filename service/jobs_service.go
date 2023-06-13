// Copyright (c) 2022-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package service

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"runtime"
	"time"

	"github.com/mattermost/calls-offloader/service/random"
	recorder "github.com/mattermost/calls-recorder/cmd/recorder/config"

	"github.com/mattermost/mattermost/server/public/shared/mlog"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	docker "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

const dockerRequestTimeout = 10 * time.Second
const dockerImagePullTimeout = 2 * time.Minute
const dockerGracefulExitCode = 143

var dockerStopTimeout = 5 * time.Minute

type stopCb func(job Job, exitCode int) error

type JobService struct {
	log *mlog.Logger
	cfg JobsConfig
}

func NewJobService(cfg JobsConfig, log *mlog.Logger) (*JobService, error) {
	if err := cfg.IsValid(); err != nil {
		return nil, fmt.Errorf("failed to validate config: %w", err)
	}

	switch cfg.APIType {
	case JobAPITypeDocker:
		version, err := getServerVersionDocker()
		if err != nil {
			return nil, fmt.Errorf("failed to get docker server version: %w", err)
		}
		log.Info("connected to docker API",
			mlog.String("version", version.Version),
			mlog.String("api_version", version.APIVersion),
		)
	default:
		return nil, fmt.Errorf("%s API is not implemeneted", cfg.APIType)
	}

	return &JobService{
		log: log,
		cfg: cfg,
	}, nil
}

func (s *JobService) UpdateJobRunnerDocker(runner string) error {
	ctx, cancel := context.WithTimeout(context.Background(), dockerRequestTimeout)
	defer cancel()
	cli, err := docker.NewClientWithOpts(docker.FromEnv, docker.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("failed to create docker client: %w", err)
	}
	defer cli.Close()

	// We check whether the runner (docker image) exists already. If not we try
	// and pull it from the public registry. This outer check is especially useful
	// when running things locally where there's no registry.
	if _, _, err := cli.ImageInspectWithRaw(ctx, runner); err != nil {
		// cancelling existing context as pulling the image may take a while.
		cancel()

		imagePullCtx, cancel := context.WithTimeout(context.Background(), dockerImagePullTimeout)
		defer cancel()
		s.log.Debug("image is missing, will try to pull it from registry")
		out, err := cli.ImagePull(imagePullCtx, runner, types.ImagePullOptions{})
		if err != nil {
			return fmt.Errorf("failed to pull docker image: %w", err)
		}
		defer out.Close()

		scanner := bufio.NewScanner(out)
		for scanner.Scan() {
			s.log.Debug(scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("failed to scan output: %w", err)
		}
	}

	return nil
}

func (s *JobService) CreateRecordingJobDocker(cfg JobConfig, onStopCb stopCb) (Job, error) {
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

	// We fetch the list of running containers to check against it in order to
	// ensure we don't exceed the configured MaxConcurrentJobs limit.
	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{})
	if err != nil {
		return Job{}, fmt.Errorf("failed to list containers: %w", err)
	}

	devMode := os.Getenv("DEV_MODE") == "true"

	if len(containers) >= s.cfg.MaxConcurrentJobs {
		if !devMode {
			return Job{}, fmt.Errorf("max concurrent jobs reached")
		}
		s.log.Warn("max concurrent jobs reached", mlog.Int("number of active containers", len(containers)),
			mlog.Int("cfg.MaxConcurrentJobs", s.cfg.MaxConcurrentJobs))
	}

	if err := s.UpdateJobRunnerDocker(job.Runner); err != nil {
		return Job{}, fmt.Errorf("failed to update job runner: %w", err)
	}

	var jobData recorder.RecorderConfig
	jobData.FromMap(cfg.InputData)
	jobData.SetDefaults()

	var networkMode container.NetworkMode
	var env []string
	if devMode {
		env = append(env, "DEV_MODE=true")
		job.Runner = "calls-recorder:master"
		if runtime.GOOS == "linux" {
			networkMode = "host"
		}
		if runtime.GOOS == "darwin" {
			u, err := url.Parse(jobData.SiteURL)
			if err == nil && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1") {
				u.Host = "host.docker.internal" + ":" + u.Port()
				jobData.SiteURL = u.String()
			}
		}
	}

	if dockerNetwork := os.Getenv("DOCKER_NETWORK"); dockerNetwork != "" {
		networkMode = container.NetworkMode(dockerNetwork)
	}

	env = append(env, jobData.ToEnv()...)

	volumeID := "calls-recorder-" + random.NewID()
	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image:   job.Runner,
		Tty:     false,
		Env:     env,
		Volumes: map[string]struct{}{volumeID + ":/recs": {}},
	}, &container.HostConfig{
		NetworkMode: networkMode,
		Mounts: []mount.Mount{
			{
				Target: "/recs",
				Source: volumeID,
				Type:   "volume",
			},
		},
		SecurityOpt: []string{dockerSecurityOpts},
	}, nil, nil, "")
	if err != nil {
		return Job{}, fmt.Errorf("failed to create container: %w", err)
	}

	job.ID = resp.ID[:12]

	if err := cli.ContainerStart(ctx, job.ID, types.ContainerStartOptions{}); err != nil {
		return Job{}, fmt.Errorf("failed to start container: %w", err)
	}

	job.StartAt = time.Now().UnixMilli()

	// We wait for the container to exit to cover both the case of unexpected error or
	// the execution reaching the configured MaxDurationSec. The provided callback is used
	// to update the caller about this occurrence.
	go func() {
		timeout := dockerRequestTimeout
		if cfg.MaxDurationSec > 0 {
			timeout = time.Duration(cfg.MaxDurationSec) * time.Second
		}

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		waitCh, errCh := cli.ContainerWait(ctx, job.ID, container.WaitConditionNotRunning)

		var exitCode int
		select {
		case res := <-waitCh:
			exitCode = int(res.StatusCode)
			s.log.Debug("container exited", mlog.String("jobID", job.ID), mlog.Int("exitCode", exitCode))
		case err := <-errCh:
			s.log.Warn("timeout reached, stopping job", mlog.Err(err), mlog.String("jobID", job.ID))
			if err := s.StopRecordingJobDocker(job.ID); err != nil {
				s.log.Error("failed to stop job", mlog.Err(err), mlog.String("jobID", job.ID))
				return
			}

			ctx, cancel := context.WithTimeout(context.Background(), dockerRequestTimeout)
			defer cancel()
			cnt, err := cli.ContainerInspect(ctx, job.ID)
			if err != nil {
				s.log.Error("failed to inspect container", mlog.Err(err), mlog.String("jobID", job.ID))
				return
			}

			if cnt.State == nil {
				s.log.Error("container state is missing", mlog.String("jobID", job.ID))
				return
			}

			exitCode = cnt.State.ExitCode
		}

		if err := onStopCb(job, exitCode); err != nil {
			s.log.Error("failed to run onStopCb", mlog.Err(err), mlog.String("jobID", job.ID))
		}
	}()

	return job, nil
}

func (s *JobService) StopRecordingJobDocker(jobID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), dockerStopTimeout)
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

func (s *JobService) RemoveRecordingJobDocker(jobID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), dockerRequestTimeout)
	defer cancel()
	cli, err := docker.NewClientWithOpts(docker.FromEnv, docker.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("failed to create docker client: %w", err)
	}
	defer cli.Close()

	cnt, err := cli.ContainerInspect(ctx, jobID)
	if err != nil {
		return fmt.Errorf("failed to get container: %w", err)
	}

	if err := cli.ContainerRemove(ctx, jobID, types.ContainerRemoveOptions{}); err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}

	if len(cnt.Mounts) == 0 {
		return fmt.Errorf("container should have one volume")
	}

	if err := cli.VolumeRemove(ctx, cnt.Mounts[0].Name, false); err != nil {
		return fmt.Errorf("failed to remove volume: %w", err)
	}

	return nil
}
