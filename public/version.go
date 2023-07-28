// Copyright (c) 2022-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package public

import (
	"github.com/mattermost/mattermost/server/public/shared/mlog"
)

type VersionInfo struct {
	BuildDate    string `json:"buildDate"`
	BuildVersion string `json:"buildVersion"`
	BuildHash    string `json:"buildHash"`
	GoVersion    string `json:"goVersion"`
}

func (v VersionInfo) LogFields() []mlog.Field {
	return []mlog.Field{
		mlog.String("buildDate", v.BuildDate),
		mlog.String("buildVersion", v.BuildVersion),
		mlog.String("buildHash", v.BuildHash),
		mlog.String("goVersion", v.GoVersion),
	}
}
