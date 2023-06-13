// Copyright (c) 2022-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package service

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/mattermost/mattermost/server/public/shared/mlog"
)

type httpData struct {
	err      string
	code     int
	clientID string
	reqData  map[string]string
	resData  map[string]any
}

func newHTTPData() *httpData {
	return &httpData{
		reqData: map[string]string{},
		resData: map[string]any{},
	}
}

func (s *Service) httpAudit(handler string, data *httpData, w http.ResponseWriter, r *http.Request) {
	fields := append(reqAuditFields(r), mlog.Int("code", data.code))
	status := "fail"
	if data.err == "" {
		status = "success"
	} else {
		data.resData["error"] = data.err
		fields = append(fields, mlog.Err(fmt.Errorf("%s", data.err)))
	}

	clientID := data.clientID
	if cID := data.reqData["clientID"]; cID != "" {
		clientID = cID
	}
	fields = append(fields, mlog.String("clientID", clientID))

	s.log.Debug(handler, append(fields, mlog.String("status", status))...)
	if w != nil && len(data.resData) > 0 {
		data.resData["code"] = fmt.Sprintf("%d", data.code)
		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(data.code)
		if err := json.NewEncoder(w).Encode(data.resData); err != nil {
			s.log.Error("failed to encode data", mlog.Err(err))
		}
	}
}

func reqAuditFields(req *http.Request) []mlog.Field {
	hdr := req.Header.Clone()
	delete(hdr, "Authorization")
	fields := []mlog.Field{
		mlog.String("remoteAddr", req.RemoteAddr),
		mlog.String("method", req.Method),
		mlog.String("url", req.URL.String()),
		mlog.Any("header", hdr),
		mlog.String("host", req.Host),
	}
	return fields
}
