// Copyright (c) 2022-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package service

import (
	"fmt"

	"github.com/mattermost/calls-offloader/service/docker"
	"github.com/mattermost/calls-offloader/service/job"

	"github.com/mattermost/mattermost-server/v6/shared/mlog"
)

const gracefulExitCode = 143

type JobService interface {
	UpdateJobRunner(runner string) error
	CreateJob(cfg job.Config, onStopCb job.StopCb) (job.Job, error)
	StopJob(jobID string) error
	DeleteJob(jobID string) error
	GetJobLogs(jobID string) ([]byte, error)
}

func NewJobService(cfg JobsConfig, log mlog.LoggerIFace) (JobService, error) {
	if err := cfg.IsValid(); err != nil {
		return nil, fmt.Errorf("failed to validate config: %w", err)
	}

	switch cfg.APIType {
	case JobAPITypeDocker:
		return docker.NewJobService(log, docker.JobServiceConfig{
			MaxConcurrentJobs: cfg.MaxConcurrentJobs,
		})
	case JobAPITypeKubernetes:
		return nil, fmt.Errorf("%s API is not implemeneted", cfg.APIType)
	default:
		return nil, fmt.Errorf("%s API is not implemeneted", cfg.APIType)
	}
}
