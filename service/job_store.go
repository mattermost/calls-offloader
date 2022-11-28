// Copyright (c) 2022-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package service

import (
	"encoding/json"
	"fmt"
)

const jobKeyPrefix = "job_"

func (s *Service) SaveJob(job Job) error {
	js, err := json.Marshal(&job)
	if err != nil {
		return fmt.Errorf("failed to marshal: %w", err)
	}
	if err := s.store.Set(jobKeyPrefix+job.ID, string(js)); err != nil {
		return fmt.Errorf("failed to save to store: %w", err)
	}
	return nil
}

func (s *Service) GetJob(jobID string) (Job, error) {
	js, err := s.store.Get(jobKeyPrefix + jobID)
	if err != nil {
		return Job{}, fmt.Errorf("failed to get job: %w", err)
	}

	var job Job
	if err := json.Unmarshal([]byte(js), &job); err != nil {
		return Job{}, fmt.Errorf("failed to unmarshal: %w", err)
	}

	return job, nil
}

func (s *Service) DeleteJob(jobID string) error {
	if err := s.store.Delete(jobKeyPrefix + jobID); err != nil {
		return fmt.Errorf("failed to delete job: %w", err)
	}
	return nil
}
