// Copyright (c) 2022-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package job

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

type Type string

const (
	TypeRecording    Type = "recording"
	TypeTranscribing Type = "transcribing"
)

const (
	MinSupportedRecorderVersion    = "0.6.0"
	MinSupportedTranscriberVersion = "0.1.0"
	RecordingJobPrefix             = "calls-recorder"
	TranscribingJobPrefix          = "calls-transcriber"
	ImageRegistryDefault           = "mattermost"

	InputDataSiteURLKey = "site_url"
)

type ServiceConfig struct {
	Runners []string
}

type Job struct {
	Config
	ID         string         `json:"id"`
	StartAt    int64          `json:"start_at"`
	StopAt     int64          `json:"stop_at,omitempty"`
	OutputData map[string]any `json:"output_data,omitempty"`
}

type InputData map[string]any

func (d InputData) GetSiteURL() string {
	if d == nil {
		return ""
	}
	siteURL, _ := d[InputDataSiteURLKey].(string)
	return siteURL
}

func (d InputData) SetSiteURL(siteURL string) {
	if d == nil {
		return
	}
	d[InputDataSiteURLKey] = siteURL
}

func (d InputData) ToEnv() []string {
	env := make([]string, 0, len(d))
	for k, v := range d {
		env = append(env, fmt.Sprintf("%s=%v", strings.ToUpper(k), v))
	}
	return env
}

type Config struct {
	Type           Type      `json:"type"`
	MaxDurationSec int64     `json:"max_duration_sec"`
	Runner         string    `json:"runner"`
	InputData      InputData `json:"input_data,omitempty"`
}

type StopCb func(job Job, success bool) error

func (c ServiceConfig) IsValid(registry string) error {
	if len(c.Runners) == 0 {
		return fmt.Errorf("invalid empty Runners")
	}

	if registry == "" {
		return fmt.Errorf("registry should not be empty")
	}

	for _, runner := range c.Runners {
		if err := RunnerIsValid(runner, registry); err != nil {
			return err
		}
	}
	return nil
}

func RunnerIsValid(runner, registry string) error {
	if os.Getenv("DEV_MODE") == "true" || os.Getenv("TEST_MODE") == "true" {
		return nil
	}

	if runner == "" {
		return fmt.Errorf("runner should not be empty")
	}

	if registry == "" {
		return fmt.Errorf("registry should not be empty")
	}

	recorderRunnerREs := []*regexp.Regexp{
		regexp.MustCompile(fmt.Sprintf(`^%s/%s:v((?:0|[1-9]\d*)\.(?:0|[1-9]\d*)\.(?:0|[1-9]\d*))(?:-dev\d*)?$`, registry, RecordingJobPrefix)),
	}
	transcriberRunnerREs := []*regexp.Regexp{
		regexp.MustCompile(fmt.Sprintf(`^%s/%s:v((?:0|[1-9]\d*)\.(?:0|[1-9]\d*)\.(?:0|[1-9]\d*))(?:-dev\d*)?$`, registry, TranscribingJobPrefix)),
	}

	for _, re := range recorderRunnerREs {
		if matches := re.FindStringSubmatch(runner); len(matches) > 1 {
			return checkMinVersion(MinSupportedRecorderVersion, matches[1])
		}
	}

	for _, re := range transcriberRunnerREs {
		if matches := re.FindStringSubmatch(runner); len(matches) > 1 {
			return checkMinVersion(MinSupportedTranscriberVersion, matches[1])
		}
	}

	return fmt.Errorf("failed to validate runner %q", runner)
}

func (c Config) IsValid(registry string) error {
	if c.Type == "" {
		return fmt.Errorf("invalid Type value: should not be empty")
	}

	if err := RunnerIsValid(c.Runner, registry); err != nil {
		return fmt.Errorf("invalid Runner value: %w", err)
	}

	if c.MaxDurationSec <= 0 {
		return fmt.Errorf("invalid MaxDurationSec value: should be positive")
	}

	switch c.Type {
	case TypeRecording:
	case TypeTranscribing:
	default:
		return fmt.Errorf("invalid Type value: %q", c.Type)
	}

	// Specific job config validation is deferred to the client side (e.g. plugin)
	// and to job process itself in order to avoid coupling configs with this service.

	return nil
}
