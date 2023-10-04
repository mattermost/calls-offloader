// Copyright (c) 2022-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package service

import (
	"fmt"
	"io"

	"github.com/mattermost/calls-offloader/public/job"
	"github.com/mattermost/calls-offloader/service/docker"
	"github.com/mattermost/calls-offloader/service/kubernetes"

	"github.com/mattermost/mattermost/server/public/shared/mlog"
)

type JobService interface {
	Init(cfg job.ServiceConfig) error
	CreateJob(cfg job.Config, onStopCb job.StopCb) (job.Job, error)
	DeleteJob(jobID string) error
	GetJobLogs(jobID string, stdout, stderr io.Writer) error
	Shutdown() error
}

func NewJobService(cfg JobsConfig, log mlog.LoggerIFace) (JobService, error) {
	if err := cfg.IsValid(); err != nil {
		return nil, fmt.Errorf("failed to validate config: %w", err)
	}

	switch cfg.APIType {
	case JobAPITypeDocker:
		return docker.NewJobService(log, docker.JobServiceConfig{
			MaxConcurrentJobs:       cfg.MaxConcurrentJobs,
			FailedJobsRetentionTime: cfg.FailedJobsRetentionTime.Duration(),
		})
	case JobAPITypeKubernetes:
		return kubernetes.NewJobService(log, kubernetes.JobServiceConfig{
			MaxConcurrentJobs:       cfg.MaxConcurrentJobs,
			FailedJobsRetentionTime: cfg.FailedJobsRetentionTime.Duration(),
		})
	default:
		return nil, fmt.Errorf("%s API is not implemeneted", cfg.APIType)
	}
}
