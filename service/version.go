// Copyright (c) 2022-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package service

import (
	"encoding/json"
	"net/http"
	"runtime"

	"github.com/mattermost/calls-offloader/public"

	"github.com/mattermost/mattermost/server/public/shared/mlog"
)

var (
	buildVersion string
	buildHash    string
	buildDate    string
)

func getVersionInfo() public.VersionInfo {
	return public.VersionInfo{
		BuildDate:    buildDate,
		BuildVersion: buildVersion,
		BuildHash:    buildHash,
		GoVersion:    runtime.Version(),
	}
}

func (s *Service) getVersion(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.NotFound(w, req)
		return
	}

	w.Header().Add("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(getVersionInfo()); err != nil {
		s.log.Error("failed to encode data", mlog.Err(err))
	}
}
