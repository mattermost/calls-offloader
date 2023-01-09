// Copyright (c) 2022-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"

	"github.com/mattermost/mattermost-server/v6/shared/mlog"
)

func (s *Service) handleCreateJob(w http.ResponseWriter, r *http.Request) {
	data := newHTTPData()
	defer s.httpAudit("handleCreateJob", data, w, r)

	clientID, code, err := s.authHandler(w, r)
	if err != nil {
		data.err = err.Error()
		data.code = code
		return
	}
	data.clientID = clientID

	var cfg JobConfig
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, apiRequestBodyMaxSizeBytes)).Decode(&cfg); err != nil {
		data.err = "failed to decode request body: " + err.Error()
		data.code = http.StatusBadRequest
		return
	}

	if err := cfg.IsValid(); err != nil {
		data.err = err.Error()
		data.code = http.StatusBadRequest
		return
	}

	if s.cfg.Jobs.APIType != JobAPITypeDocker || cfg.Type != JobTypeRecording {
		data.err = "not implemented"
		data.code = http.StatusNotImplemented
		return
	}

	job, err := s.jobService.CreateRecordingJobDocker(cfg, func(job Job, exitCode int) error {
		s.log.Info("job stopped", mlog.String("jobID", job.ID), mlog.Int("exitCode", exitCode))

		job, err := s.GetJob(job.ID)
		if err != nil {
			return err
		}

		if job.StopAt == 0 {
			job.StopAt = time.Now().UnixMilli()
			if err := s.SaveJob(job); err != nil {
				return err
			}
		}

		if exitCode == dockerGracefulExitCode {
			s.log.Debug("job completed successfully, removing",
				mlog.String("jobID", job.ID), mlog.Int("exitCode", exitCode))
			if err := s.jobService.RemoveRecordingJobDocker(job.ID); err != nil {
				return fmt.Errorf("failed to remove recording job: %w", err)
			}
			if err := s.DeleteJob(job.ID); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		data.err = "failed to create recording job: " + err.Error()
		data.code = http.StatusInternalServerError
		return
	}

	if err := s.SaveJob(job); err != nil {
		data.err = "failed to save job: " + err.Error()
		data.code = http.StatusInternalServerError
		return
	}

	data.code = http.StatusOK

	if err := json.NewEncoder(w).Encode(job); err != nil {
		s.log.Error("failed to encode response", mlog.Err(err))
	}
}

func (s *Service) handleGetJob(w http.ResponseWriter, r *http.Request) {
	data := newHTTPData()
	defer s.httpAudit("handleGetJob", data, w, r)
	clientID, code, err := s.authHandler(w, r)
	if err != nil {
		data.err = err.Error()
		data.code = code
		return
	}
	data.clientID = clientID

	jobID := mux.Vars(r)["id"]
	if jobID == "" {
		data.err = "missing job ID"
		data.code = http.StatusBadRequest
		return
	}

	job, err := s.GetJob(jobID)
	if err != nil {
		data.err = "failed to get job " + err.Error()
		data.code = http.StatusNotFound
		return
	}

	data.code = http.StatusOK

	if err := json.NewEncoder(w).Encode(job); err != nil {
		s.log.Error("failed to encode response", mlog.Err(err))
	}
}

func (s *Service) handleStopJob(w http.ResponseWriter, r *http.Request) {
	data := newHTTPData()
	defer s.httpAudit("handleStopJob", data, w, r)

	clientID, code, err := s.authHandler(w, r)
	if err != nil {
		data.err = err.Error()
		data.code = code
		return
	}
	data.clientID = clientID

	jobID := mux.Vars(r)["id"]
	if jobID == "" {
		data.err = "missing job ID"
		data.code = http.StatusBadRequest
		return
	}

	job, err := s.GetJob(jobID)
	if err != nil {
		data.err = "failed to get job " + err.Error()
		data.code = http.StatusNotFound
		return
	}

	err = s.jobService.StopRecordingJobDocker(jobID)
	if err != nil {
		data.err = "failed to stop recording job: " + err.Error()
		data.code = http.StatusInternalServerError
		return
	}

	job.StopAt = time.Now().UnixMilli()
	if err := s.SaveJob(job); err != nil {
		data.err = "failed to save job: " + err.Error()
		data.code = http.StatusInternalServerError
		return
	}

	data.code = http.StatusOK
}

func (s *Service) handleJobGetLogs(w http.ResponseWriter, r *http.Request) {
	data := newHTTPData()
	defer s.httpAudit("handleJobGetLogs", data, w, r)

	clientID, code, err := s.authHandler(w, r)
	if err != nil {
		s.log.Debug("dang")
		data.err = err.Error()
		data.code = code
		return
	}
	data.clientID = clientID

	jobID := mux.Vars(r)["id"]
	if jobID == "" {
		data.err = "missing job ID"
		data.code = http.StatusBadRequest
		return
	}

	logs, err := s.jobService.RecordingJobLogsDocker(jobID)
	if err != nil {
		data.err = "failed to get recording job logs: " + err.Error()
		data.code = http.StatusForbidden
		return
	}

	data.code = http.StatusOK
	if _, err := w.Write(logs); err != nil {
		s.log.Error("failed to write response", mlog.Err(err))
	}
}

func (s *Service) handleDeleteJob(w http.ResponseWriter, r *http.Request) {
	data := newHTTPData()
	defer s.httpAudit("handleRemoveJob", data, w, r)

	clientID, code, err := s.authHandler(w, r)
	if err != nil {
		data.err = err.Error()
		data.code = code
		return
	}
	data.clientID = clientID

	jobID := mux.Vars(r)["id"]
	if jobID == "" {
		data.err = "missing job ID"
		data.code = http.StatusBadRequest
		return
	}

	job, err := s.GetJob(jobID)
	if err != nil {
		data.err = "failed to get job " + err.Error()
		data.code = http.StatusNotFound
		return
	}

	// TODO: consider adding a force removal option to cover edge cases.
	if job.StopAt == 0 {
		data.err = "job is running"
		data.code = http.StatusBadRequest
		return
	}

	err = s.jobService.RemoveRecordingJobDocker(jobID)
	if err != nil {
		data.err = "failed to remove recording job: " + err.Error()
		data.code = http.StatusInternalServerError
		return
	}

	data.code = http.StatusOK
}

func (s *Service) handleUpdateJobRunner(w http.ResponseWriter, r *http.Request) {
	data := newHTTPData()
	defer s.httpAudit("handleUpdateJobRunner", data, w, r)

	clientID, code, err := s.authHandler(w, r)
	if err != nil {
		data.err = err.Error()
		data.code = code
		return
	}
	data.clientID = clientID

	var info map[string]interface{}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, apiRequestBodyMaxSizeBytes)).Decode(&info); err != nil {
		data.err = "failed to decode request body: " + err.Error()
		data.code = http.StatusBadRequest
		return
	}

	runner, ok := info["runner"].(string)
	if !ok || runner == "" {
		data.err = "invalid request body"
		data.code = http.StatusBadRequest
		return
	}

	if err := s.jobService.UpdateJobRunnerDocker(runner); err != nil {
		data.err = "failed to update job runner: " + err.Error()
		data.code = http.StatusInternalServerError
		return
	}

	if err != nil {
		data.err = "failed to create recording job: " + err.Error()
		data.code = http.StatusInternalServerError
		return
	}

	data.code = http.StatusOK
}
