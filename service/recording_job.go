// Copyright (c) 2022-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package service

import (
	"fmt"
)

type RecordingJobInputData struct {
	SiteURL   string `json:"site_url"`
	CallID    string `json:"call_id"`
	ThreadID  string `json:"thread_id"`
	AuthToken string `json:"auth_token"`
}

func (c RecordingJobInputData) IsValid() error {
	if c.SiteURL == "" {
		return fmt.Errorf("invalid SiteURL value: should not be empty")
	}

	if c.CallID == "" {
		return fmt.Errorf("invalid CallID value: should not be empty")
	}

	if c.ThreadID == "" {
		return fmt.Errorf("invalid ThreadID be empty")
	}

	if c.AuthToken == "" {
		return fmt.Errorf("invalid AuthToken value: should not be empty")
	}

	return nil
}

func (c *RecordingJobInputData) ToMap() map[string]any {
	return map[string]any{
		"site_url":   c.SiteURL,
		"call_id":    c.CallID,
		"thread_id":  c.ThreadID,
		"auth_token": c.AuthToken,
	}
}

func (c *RecordingJobInputData) FromMap(m map[string]any) *RecordingJobInputData {
	c.SiteURL, _ = m["site_url"].(string)
	c.CallID, _ = m["call_id"].(string)
	c.ThreadID, _ = m["thread_id"].(string)
	c.AuthToken, _ = m["auth_token"].(string)
	return c
}

func (c *RecordingJobInputData) ToEnv() []string {
	return []string{
		fmt.Sprintf("SITE_URL=%s", c.SiteURL),
		fmt.Sprintf("CALL_ID=%s", c.CallID),
		fmt.Sprintf("THREAD_ID=%s", c.ThreadID),
		fmt.Sprintf("AUTH_TOKEN=%s", c.AuthToken),
	}
}
