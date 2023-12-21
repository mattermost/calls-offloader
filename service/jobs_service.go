// Copyright (c) 2022-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package service

import (
	"fmt"
	"io"
	"time"

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
		cfg.Docker.MaxConcurrentJobs = cfg.MaxConcurrentJobs
		cfg.Docker.FailedJobsRetentionTime = time.Duration(cfg.FailedJobsRetentionTime)
		log.Info("creating new job service", mlog.Any("apiType", cfg.APIType), mlog.String("config", fmt.Sprintf("%+v", cfg.Docker)))
		return docker.NewJobService(log, cfg.Docker)
	case JobAPITypeKubernetes:
		cfg.Kubernetes.MaxConcurrentJobs = cfg.MaxConcurrentJobs
		cfg.Kubernetes.FailedJobsRetentionTime = time.Duration(cfg.FailedJobsRetentionTime)
		log.Info("creating new job service", mlog.Any("apiType", cfg.APIType), mlog.String("config", fmt.Sprintf("%+v", cfg.Kubernetes)))
		return kubernetes.NewJobService(log, cfg.Kubernetes)
	default:
		return nil, fmt.Errorf("%s API is not implemeneted", cfg.APIType)
	}
}
