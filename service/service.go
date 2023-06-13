// Copyright (c) 2022-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package service

import (
	"fmt"
	"net/http/pprof"

	"github.com/mattermost/calls-offloader/logger"
	"github.com/mattermost/calls-offloader/service/api"
	"github.com/mattermost/calls-offloader/service/auth"
	"github.com/mattermost/calls-offloader/service/store"

	"github.com/mattermost/mattermost/server/public/shared/mlog"

	"github.com/gorilla/mux"
)

const apiRequestBodyMaxSizeBytes = 1024 * 1024 // 1MB

type Service struct {
	cfg          Config
	apiServer    *api.Server
	store        store.Store
	auth         *auth.Service
	log          *mlog.Logger
	jobService   *JobService
	sessionCache *auth.SessionCache
}

func New(cfg Config) (*Service, error) {
	if err := cfg.IsValid(); err != nil {
		return nil, err
	}

	s := &Service{
		cfg: cfg,
	}

	var err error
	s.log, err = logger.New(cfg.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to init logger: %w", err)
	}

	s.log.Info("starting up", getVersionInfo().logFields()...)

	s.store, err = store.New(cfg.Store.DataSource)
	if err != nil {
		return nil, fmt.Errorf("failed to create store: %w", err)
	}
	s.log.Info("initiated data store", mlog.String("DataSource", cfg.Store.DataSource))

	s.sessionCache, err = auth.NewSessionCache(cfg.API.Security.SessionCache)
	if err != nil {
		return nil, fmt.Errorf("failed to create session cache: %w", err)
	}

	s.auth, err = auth.NewService(s.store, s.sessionCache)
	if err != nil {
		return nil, fmt.Errorf("failed to create auth service: %w", err)
	}
	s.log.Info("initiated auth service")

	s.apiServer, err = api.NewServer(cfg.API.HTTP, s.log)
	if err != nil {
		return nil, fmt.Errorf("failed to create api server: %w", err)
	}

	s.jobService, err = NewJobService(cfg.Jobs, s.log)
	if err != nil {
		return nil, fmt.Errorf("failed to create job service: %w", err)
	}
	s.log.Info("initiated job service")

	router := mux.NewRouter()
	router.HandleFunc("/version", s.getVersion)
	router.HandleFunc("/login", s.loginClient)
	router.HandleFunc("/register", s.registerClient)
	router.HandleFunc("/unregister", s.unregisterClient)
	router.HandleFunc("/jobs", s.handleCreateJob).Methods("POST")
	router.HandleFunc("/jobs/{id:[a-z0-9]{12}}/stop", s.handleStopJob).Methods("POST")
	router.HandleFunc("/jobs/{id:[a-z0-9]{12}}/logs", s.handleJobGetLogs).Methods("GET")
	router.HandleFunc("/jobs/{id:[a-z0-9]{12}}", s.handleGetJob).Methods("GET")
	router.HandleFunc("/jobs/{id:[a-z0-9]{12}}", s.handleDeleteJob).Methods("DELETE")
	router.HandleFunc("/jobs/update-runner", s.handleUpdateJobRunner).Methods("POST")

	router.Handle("/debug/pprof/heap", pprof.Handler("heap"))
	router.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))
	router.Handle("/debug/pprof/mutex", pprof.Handler("mutex"))
	router.HandleFunc("/debug/pprof/profile", pprof.Profile)
	router.HandleFunc("/debug/pprof/trace", pprof.Trace)

	s.apiServer.RegisterHandler("/", router)

	return s, nil
}

func (s *Service) Start() error {
	if err := s.apiServer.Start(); err != nil {
		return fmt.Errorf("failed to start api server: %w", err)
	}
	return nil
}

func (s *Service) Stop() error {
	s.log.Info("shutting down")

	if err := s.apiServer.Stop(); err != nil {
		return fmt.Errorf("failed to stop api server: %w", err)
	}

	if err := s.store.Close(); err != nil {
		return fmt.Errorf("failed to close store: %w", err)
	}

	if err := s.log.Shutdown(); err != nil {
		return fmt.Errorf("failed to shutdown logger: %w", err)
	}

	return nil
}
