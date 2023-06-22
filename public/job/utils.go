// Copyright (c) 2022-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package job

import (
	"fmt"

	"github.com/Masterminds/semver"
)

func checkMinVersion(minVersion, actualVersion string) error {
	minV, err := semver.NewVersion(minVersion)
	if err != nil {
		return fmt.Errorf("failed to parse minVersion: %w", err)
	}

	currV, err := semver.NewVersion(actualVersion)
	if err != nil {
		return fmt.Errorf("failed to parse actualVersion: %w", err)
	}

	if cmp := currV.Compare(minV); cmp < 0 {
		return fmt.Errorf("actual version (%s) is lower than minimum supported version (%s)", actualVersion, minVersion)
	}

	return nil
}
