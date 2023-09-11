// Copyright (c) 2022-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package docker

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"time"

	"github.com/mattermost/calls-offloader/public/job"
	"github.com/mattermost/calls-offloader/service/random"

	recorder "github.com/mattermost/calls-recorder/cmd/recorder/config"
	transcriber "github.com/mattermost/calls-transcriber/cmd/transcriber/config"

	"github.com/mattermost/mattermost/server/public/shared/mlog"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	docker "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

const (
	dockerRequestTimeout   = 10 * time.Second
	dockerImagePullTimeout = 2 * time.Minute
	dockerVolumePath       = "/data"
)

const (
	recordingJobPrefix    = "calls-recorder"
	transcribingJobPrefix = "calls-transcriber"
)

var (
	dockerStopTimeout          = 5 * time.Minute
	dockerRetentionJobInterval = time.Minute
)

type JobServiceConfig struct {
	MaxConcurrentJobs       int
	FailedJobsRetentionTime time.Duration
}

type JobService struct {
	cfg JobServiceConfig
	log mlog.LoggerIFace

	client             *docker.Client
	stopCh             chan struct{}
	retentionJobDoneCh chan struct{}
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

	s := &JobService{
		cfg:                cfg,
		log:                log,
		client:             client,
		stopCh:             make(chan struct{}),
		retentionJobDoneCh: make(chan struct{}),
	}

	if s.cfg.FailedJobsRetentionTime > 0 {
		go s.retentionJob()
	} else {
		s.log.Info("skipping retention job", mlog.Any("retention_time", s.cfg.FailedJobsRetentionTime))
		close(s.retentionJobDoneCh)
	}

	return s, nil
}

func (s *JobService) retentionJob() {
	s.log.Info("retention job is starting",
		mlog.Any("retention_time", s.cfg.FailedJobsRetentionTime),
	)
	defer func() {
		s.log.Info("exiting retention job")
		close(s.retentionJobDoneCh)
	}()

	ticker := time.NewTicker(dockerRetentionJobInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), dockerRequestTimeout)
			containers, err := s.client.ContainerList(ctx, types.ContainerListOptions{
				All: true,
				Filters: filters.NewArgs(filters.KeyValuePair{
					Key:   "status",
					Value: "exited",
				}, filters.KeyValuePair{
					Key:   "label",
					Value: "app=mattermost-calls-offloader",
				}),
			})
			cancel()
			if err != nil {
				s.log.Error("failed to list containers", mlog.Err(err))
				continue
			}

			if len(containers) == 0 {
				// nothing to do
				continue
			}

			for _, cnt := range containers {
				ctx, cancel = context.WithTimeout(context.Background(), dockerRequestTimeout)
				c, err := s.client.ContainerInspect(ctx, cnt.ID)
				cancel()
				if err != nil {
					s.log.Error("failed to get container", mlog.Err(err))
					continue
				}

				if c.State == nil {
					s.log.Error("container state is missing", mlog.String("id", cnt.ID))
					continue
				}

				finishedAt, err := time.Parse(time.RFC3339, c.State.FinishedAt)
				if err != nil {
					s.log.Error("failed to parse finish time", mlog.Err(err))
					continue
				}

				if since := time.Since(finishedAt); since > s.cfg.FailedJobsRetentionTime {
					s.log.Info("configured retention time has elapsed since the container finished, deleting",
						mlog.String("id", cnt.ID),
						mlog.Any("retention_time", s.cfg.FailedJobsRetentionTime),
						mlog.Any("finish_at", finishedAt),
						mlog.Any("since", since),
					)

					if err := s.DeleteJob(cnt.ID); err != nil {
						s.log.Error("failed to delete job", mlog.Err(err), mlog.String("jobID", cnt.ID))
						continue
					}
				}
			}
		}
	}
}

func (s *JobService) Shutdown() error {
	s.log.Info("docker job service shutting down")

	close(s.stopCh)
	<-s.retentionJobDoneCh

	return s.client.Close()
}

func (s *JobService) Init(cfg job.ServiceConfig) error {
	errCh := make(chan error, len(cfg.Runners))
	for _, runner := range cfg.Runners {
		go func(r string) {
			errCh <- s.updateJobRunner(r)
		}(runner)
	}

	for i := 0; i < len(cfg.Runners); i++ {
		if err := <-errCh; err != nil {
			return err
		}
	}

	return nil
}

func (s *JobService) updateJobRunner(runner string) error {
	if os.Getenv("DEV_MODE") == "true" {
		runner = "calls-recorder:master"
	}

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
	if err := cfg.IsValid(); err != nil {
		return job.Job{}, fmt.Errorf("invalid job config: %w", err)
	}

	if onStopCb == nil {
		return job.Job{}, fmt.Errorf("onStopCb should not be nil")
	}

	jb := job.Job{
		Config: cfg,
	}

	devMode := os.Getenv("DEV_MODE") == "true"

	ctx, cancel := context.WithTimeout(context.Background(), dockerRequestTimeout)
	defer cancel()

	// We fetch the list of running containers to check against it in order to
	// ensure we don't exceed the configured MaxConcurrentJobs limit.
	containers, err := s.client.ContainerList(ctx, types.ContainerListOptions{
		Filters: filters.NewArgs(filters.KeyValuePair{
			Key:   "label",
			Value: "app=mattermost-calls-offloader",
		}),
	})
	if err != nil {
		return job.Job{}, fmt.Errorf("failed to list containers: %w", err)
	}
	if len(containers) >= s.cfg.MaxConcurrentJobs {
		if !devMode {
			return job.Job{}, fmt.Errorf("max concurrent jobs reached")
		}
		s.log.Warn("max concurrent jobs reached", mlog.Int("number of active containers", len(containers)),
			mlog.Int("cfg.MaxConcurrentJobs", s.cfg.MaxConcurrentJobs))
	}

	if err := s.updateJobRunner(jb.Runner); err != nil {
		return job.Job{}, fmt.Errorf("failed to update job runner: %w", err)
	}

	var env []string
	var jobPrefix string
	switch cfg.Type {
	case job.TypeRecording:
		var jobData recorder.RecorderConfig
		jobData.FromMap(cfg.InputData)
		jobData.SetDefaults()
		jobData.SiteURL = getSiteURLForJob(jobData.SiteURL)
		jobPrefix = recordingJobPrefix
		env = append(env, jobData.ToEnv()...)
	case job.TypeTranscribing:
		var jobData transcriber.CallTranscriberConfig
		jobData.FromMap(cfg.InputData)
		jobData.SetDefaults()
		jobData.SiteURL = getSiteURLForJob(jobData.SiteURL)
		jobPrefix = transcribingJobPrefix
		env = append(env, jobData.ToEnv()...)
	}

	var networkMode container.NetworkMode
	if devMode {
		env = append(env, "DEV_MODE=true")
		jb.Runner = jobPrefix + ":master"
		if runtime.GOOS == "linux" {
			networkMode = "host"
		}
	}
	if dockerNetwork := os.Getenv("DOCKER_NETWORK"); dockerNetwork != "" {
		networkMode = container.NetworkMode(dockerNetwork)
	}

	// We create a new context as updating the job runner could have taken more
	// than dockerRequestTimeout.
	ctx, cancel = context.WithTimeout(context.Background(), dockerRequestTimeout)
	defer cancel()

	volumeID := jobPrefix + "-" + random.NewID()
	resp, err := s.client.ContainerCreate(ctx, &container.Config{
		Image:   jb.Runner,
		Tty:     false,
		Env:     env,
		Volumes: map[string]struct{}{volumeID + ":" + dockerVolumePath: {}},
		Labels: map[string]string{
			// app label helps with identifying jobs.
			"app": "mattermost-calls-offloader",
		},
	}, &container.HostConfig{
		NetworkMode: networkMode,
		Mounts: []mount.Mount{
			{
				Target: dockerVolumePath,
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
			if err := s.stopJob(jb.ID); err != nil {
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

func (s *JobService) stopJob(jobID string) error {
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
