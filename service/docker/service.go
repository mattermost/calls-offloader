// Copyright (c) 2022-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package docker

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"runtime"
	"time"

	"github.com/mattermost/calls-offloader/service/job"
	"github.com/mattermost/calls-offloader/service/random"

	recorder "github.com/mattermost/calls-recorder/cmd/recorder/config"

	"github.com/mattermost/mattermost-server/v6/shared/mlog"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	docker "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

const (
	dockerRequestTimeout   = 10 * time.Second
	dockerImagePullTimeout = 2 * time.Minute
)

var (
	dockerStopTimeout = 5 * time.Minute
)

type JobServiceConfig struct {
	MaxConcurrentJobs int
}

type JobService struct {
	cfg JobServiceConfig
	log mlog.LoggerIFace

	client *docker.Client
}

func NewJobService(log mlog.LoggerIFace, cfg JobServiceConfig) (*JobService, error) {
	client, err := docker.NewClientWithOpts(docker.FromEnv, docker.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), dockerRequestTimeout)
	defer cancel()
	version, err := client.ServerVersion(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get server version: %w", err)
	}

	log.Info("connected to docker API",
		mlog.String("version", version.Version),
		mlog.String("api_version", version.APIVersion),
	)

	return &JobService{
		cfg:    cfg,
		log:    log,
		client: client,
	}, nil
}

func (s *JobService) Shutdown() error {
	s.log.Info("docker job service shutting down")
	return s.client.Close()
}

func (s *JobService) UpdateJobRunner(runner string) error {
	ctx, cancel := context.WithTimeout(context.Background(), dockerRequestTimeout)
	defer cancel()

	// We check whether the runner (docker image) exists already. If not we try
	// and pull it from the public registry. This outer check is especially useful
	// when running things locally where there's no registry.
	if _, _, err := s.client.ImageInspectWithRaw(ctx, runner); err != nil {
		// cancelling existing context as pulling the image may take a while.
		cancel()

		imagePullCtx, cancel := context.WithTimeout(context.Background(), dockerImagePullTimeout)
		defer cancel()
		s.log.Debug("image is missing, will try to pull it from registry")
		out, err := s.client.ImagePull(imagePullCtx, runner, types.ImagePullOptions{})
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

func (s *JobService) CreateJob(cfg job.Config, onStopCb job.StopCb) (job.Job, error) {
	if onStopCb == nil {
		return job.Job{}, fmt.Errorf("onStopCb should not be nil")
	}

	if cfg.Type != job.TypeRecording {
		return job.Job{}, fmt.Errorf("job type %s is not implemented", cfg.Type)
	}

	jb := job.Job{
		Config: cfg,
	}

	ctx, cancel := context.WithTimeout(context.Background(), dockerRequestTimeout)
	defer cancel()

	// We fetch the list of running containers to check against it in order to
	// ensure we don't exceed the configured MaxConcurrentJobs limit.
	containers, err := s.client.ContainerList(ctx, types.ContainerListOptions{})
	if err != nil {
		return job.Job{}, fmt.Errorf("failed to list containers: %w", err)
	}

	if len(containers) >= s.cfg.MaxConcurrentJobs {
		return job.Job{}, fmt.Errorf("max concurrent jobs reached")
	}

	if err := s.UpdateJobRunner(jb.Runner); err != nil {
		return job.Job{}, fmt.Errorf("failed to update job runner: %w", err)
	}

	var jobData recorder.RecorderConfig
	jobData.FromMap(cfg.InputData)
	jobData.SetDefaults()

	var networkMode container.NetworkMode
	var env []string
	if devMode := os.Getenv("DEV_MODE"); devMode == "true" {
		env = append(env, "DEV_MODE=true")
		jb.Runner = "calls-recorder:master"
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
	resp, err := s.client.ContainerCreate(ctx, &container.Config{
		Image:   jb.Runner,
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
		return job.Job{}, fmt.Errorf("failed to create container: %w", err)
	}

	jb.ID = resp.ID[:12]

	if err := s.client.ContainerStart(ctx, jb.ID, types.ContainerStartOptions{}); err != nil {
		return job.Job{}, fmt.Errorf("failed to start container: %w", err)
	}

	jb.StartAt = time.Now().UnixMilli()

	// We wait for the container to exit to cover both the case of unexpected error or
	// the execution reaching the configured MaxDurationSec. The provided callback is used
	// to update the caller about this occurrence.
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.MaxDurationSec)*time.Second)
		defer cancel()

		waitCh, errCh := s.client.ContainerWait(ctx, jb.ID, container.WaitConditionNotRunning)

		var exitCode int
		select {
		case res := <-waitCh:
			exitCode = int(res.StatusCode)
		case err := <-errCh:
			s.log.Warn("timeout reached, stopping job", mlog.Err(err), mlog.String("jobID", jb.ID))
			if err := s.StopJob(jb.ID); err != nil {
				s.log.Error("failed to stop job", mlog.Err(err), mlog.String("jobID", jb.ID))
				return
			}

			ctx, cancel := context.WithTimeout(context.Background(), dockerRequestTimeout)
			defer cancel()
			cnt, err := s.client.ContainerInspect(ctx, jb.ID)
			if err != nil {
				s.log.Error("failed to inspect container", mlog.Err(err), mlog.String("jobID", jb.ID))
				return
			}

			if cnt.State == nil {
				s.log.Error("container state is missing", mlog.String("jobID", jb.ID))
				return
			}

			exitCode = cnt.State.ExitCode
		}

		s.log.Debug("container exited", mlog.String("jobID", jb.ID), mlog.Int("exitCode", exitCode))

		if err := onStopCb(jb, exitCode == 0); err != nil {
			s.log.Error("failed to run onStopCb", mlog.Err(err), mlog.String("jobID", jb.ID))
		}
	}()

	return jb, nil
}

func (s *JobService) StopJob(jobID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), dockerStopTimeout)
	defer cancel()

	if err := s.client.ContainerStop(ctx, jobID, &dockerStopTimeout); err != nil {
		return fmt.Errorf("failed to stop container: %s", err.Error())
	}

	return nil
}

func (s *JobService) GetJobLogs(jobID string, stdout, stderr io.Writer) error {
	ctx, cancel := context.WithTimeout(context.Background(), dockerRequestTimeout)
	defer cancel()

	rdr, err := s.client.ContainerLogs(ctx, jobID, types.ContainerLogsOptions{
		ShowStderr: true,
		ShowStdout: true,
		Since:      time.Now().Add(-time.Hour).Format(time.RFC3339),
	})
	if err != nil {
		return fmt.Errorf("failed to get container logs: %s", err.Error())
	}
	defer rdr.Close()

	_, err = stdcopy.StdCopy(stdout, stderr, rdr)
	if err != nil {
		return fmt.Errorf("failed to read logs: %s", err.Error())
	}

	return nil
}

func (s *JobService) DeleteJob(jobID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), dockerRequestTimeout)
	defer cancel()

	cnt, err := s.client.ContainerInspect(ctx, jobID)
	if err != nil {
		return fmt.Errorf("failed to get container: %w", err)
	}

	if err := s.client.ContainerRemove(ctx, jobID, types.ContainerRemoveOptions{}); err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}

	if len(cnt.Mounts) == 0 {
		return fmt.Errorf("container should have one volume")
	}

	if err := s.client.VolumeRemove(ctx, cnt.Mounts[0].Name, false); err != nil {
		return fmt.Errorf("failed to remove volume: %w", err)
	}

	return nil
}
