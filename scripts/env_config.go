// Copyright (c) 2025-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"log"
	"os"
	"text/tabwriter"

	"github.com/mattermost/calls-offloader/service"

	"github.com/kelseyhightower/envconfig"
)

const customEnv = `
KEY                                            TYPE
K8S_NAMESPACE                                  String
  The Kubernetes namespace in which jobs will be created.
K8S_JOB_POD_TOLERATIONS                        String (JSON)
  The Kubernetes tolerations to apply to the job pods.
  Example: [{"key":"utilities","operator":"Equal","value":"true","effect":"NoSchedule"}]
`

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("unexpected number of arguments, need 1")
	}

	outFile, err := os.OpenFile(os.Args[1], os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		log.Fatalf("failed to write file: %s", err.Error())
	}
	defer outFile.Close()
	if err := outFile.Truncate(0); err != nil {
		log.Fatalf("failed to truncate file: %s", err.Error())
	}
	if _, err := outFile.Seek(0, 0); err != nil {
		log.Fatalf("failed to seek file: %s", err.Error())
	}
	fmt := "### Config Environment Overrides\n\n```\nKEY	TYPE\n{{range .}}{{usage_key .}}	{{usage_type .}}\n{{end}}```\n"
	tabs := tabwriter.NewWriter(outFile, 1, 0, 4, ' ', 0)
	_ = envconfig.Usagef("", &service.Config{}, tabs, fmt)
	tabs.Flush()

	// Custom configs
	_, err = outFile.WriteString("\n### Custom Environment Overrides\n\n```" + customEnv + "```\n")
	if err != nil {
		log.Fatalf("failed to write file: %s", err.Error())
	}
}
