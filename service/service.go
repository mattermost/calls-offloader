// Copyright (c) 2022-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package service

import (
	"fmt"
	"net/http/pprof"

	"github.com/mattermost/rtcd/logger"
	"github.com/mattermost/rtcd/service/api"
	"github.com/mattermost/rtcd/service/auth"
	"github.com/mattermost/rtcd/service/store"

	"github.com/mattermost/mattermost-server/v6/shared/mlog"
)

type Service struct {
	cfg          Config
	apiServer    *api.Server
	store        store.Store
	auth         *auth.Service
	log          *mlog.Logger
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

	s.apiServer.RegisterHandleFunc("/version", s.getVersion)
	s.apiServer.RegisterHandleFunc("/login", s.loginClient)
	s.apiServer.RegisterHandleFunc("/register", s.registerClient)
	s.apiServer.RegisterHandleFunc("/unregister", s.unregisterClient)

	s.apiServer.RegisterHandler("/debug/pprof/heap", pprof.Handler("heap"))
	s.apiServer.RegisterHandler("/debug/pprof/goroutine", pprof.Handler("goroutine"))
	s.apiServer.RegisterHandler("/debug/pprof/mutex", pprof.Handler("mutex"))
	s.apiServer.RegisterHandleFunc("/debug/pprof/profile", pprof.Profile)
	s.apiServer.RegisterHandleFunc("/debug/pprof/trace", pprof.Trace)

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
