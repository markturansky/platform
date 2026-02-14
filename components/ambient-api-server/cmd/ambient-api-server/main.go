package main

import (
	"github.com/golang/glog"

	localapi "github.com/ambient/platform/components/ambient-api-server/pkg/api"
	pkgcmd "github.com/openshift-online/rh-trex-ai/pkg/cmd"

	_ "github.com/ambient/platform/components/ambient-api-server/cmd/ambient-api-server/environments"

	_ "github.com/ambient/platform/components/ambient-api-server/plugins/users"
	_ "github.com/openshift-online/rh-trex-ai/plugins/events"
	_ "github.com/openshift-online/rh-trex-ai/plugins/generic"
	_ "github.com/ambient/platform/components/ambient-api-server/plugins/agents"
	_ "github.com/ambient/platform/components/ambient-api-server/plugins/skills"
	_ "github.com/ambient/platform/components/ambient-api-server/plugins/tasks"
	_ "github.com/ambient/platform/components/ambient-api-server/plugins/workflows"
	_ "github.com/ambient/platform/components/ambient-api-server/plugins/sessions"
	_ "github.com/ambient/platform/components/ambient-api-server/plugins/workflowSkills"
	_ "github.com/ambient/platform/components/ambient-api-server/plugins/workflowTasks"
)

func main() {
	rootCmd := pkgcmd.NewRootCommand("ambient-api-server", "Ambient API Server")
	rootCmd.AddCommand(
		pkgcmd.NewMigrateCommand("ambient-api-server"),
		pkgcmd.NewServeCommand(localapi.GetOpenAPISpec),
	)

	if err := rootCmd.Execute(); err != nil {
		glog.Fatalf("error running command: %v", err)
	}
}
