// Copyright (c) 2022-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package docker

import (
	"net/url"
	"os"
	"regexp"
	"runtime"
)

var dockerImageRE = regexp.MustCompile(`^mattermost\/(.+):v(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)$`)

func getSiteURLForJob(siteURL string) string {
	if os.Getenv("DEV_MODE") != "true" {
		return siteURL
	}

	if runtime.GOOS == "darwin" {
		u, err := url.Parse(siteURL)
		if err == nil && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1") {
			u.Host = "host.docker.internal" + ":" + u.Port()
			siteURL = u.String()
		}
	}

	return siteURL
}

func getImageNameFromRunner(runner string) string {
	matches := dockerImageRE.FindStringSubmatch(runner)
	if len(matches) != 2 {
		return ""
	}
	return matches[1]
}
