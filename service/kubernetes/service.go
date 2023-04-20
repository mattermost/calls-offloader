// Copyright (c) 2022-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package kubernetes

import (
	"fmt"
	"io"

	"github.com/mattermost/calls-offloader/service/job"

	"github.com/mattermost/mattermost-server/v6/shared/mlog"

	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type JobServiceConfig struct {
	MaxConcurrentJobs int
}

type JobService struct {
	cfg JobServiceConfig
	log mlog.LoggerIFace

	cs *k8s.Clientset
}

func NewJobService(log mlog.LoggerIFace, cfg JobServiceConfig) (*JobService, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create in-cluster config: %w", err)
	}

	cs, err := k8s.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	version, err := cs.Discovery().ServerVersion()
	if err != nil {
		return nil, fmt.Errorf("failed to get kubernetes server version: %w", err)
	}

	log.Info("connected to kubernetes API",
		mlog.String("version", fmt.Sprintf("%s.%s", version.Major, version.Minor)),
		mlog.String("git_version", version.GitVersion),
	)

	return &JobService{
		cfg: cfg,
		log: log,
		cs:  cs,
	}, nil
}

func (s *JobService) UpdateJobRunner(runner string) error {
	// May be best not to mess with k8s image pulling policy for now.
	// It's okay for images to be pulled upon first pod execution.
	return nil
}

func (s *JobService) CreateJob(cfg job.Config, onStopCb job.StopCb) (job.Job, error) {
	return job.Job{}, nil
}

func (s *JobService) StopJob(jobID string) error {
	return nil
}

func (s *JobService) DeleteJob(jobID string) error {
	return nil
}

func (s *JobService) GetJobLogs(jobID string, stdout, stderr io.Writer) error {
	return nil
}

func (s *JobService) Shutdown() error {
	return nil
}
