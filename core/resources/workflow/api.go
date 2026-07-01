package workflow

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/vanclief/agent-composer/core/controller"
	"github.com/vanclief/agent-composer/core/resources/workflow/executions"
	"github.com/vanclief/agent-composer/core/resources/workflow/nodeexecutions"
	"github.com/vanclief/agent-composer/runtime"
)

type API struct {
	Executions     *executions.API
	NodeExecutions *nodeexecutions.API
	rt             *runtime.Runtime
}

func NewAPI(ctrl *controller.Controller, rt *runtime.Runtime) *API {
	if ctrl == nil {
		panic("Controller reference is nil")
	}

	executionsAPI := executions.NewAPI(ctrl, rt)
	nodeExecutionsAPI := nodeexecutions.NewAPI(ctrl)

	return &API{
		Executions:     executionsAPI,
		NodeExecutions: nodeExecutionsAPI,
		rt:             rt,
	}
}

// DefaultShellRoot returns the effective default working directory used when a
// run does not specify a shell_root. An empty configured value resolves to the
// server's current working directory, always returned as an absolute path.
func (api *API) DefaultShellRoot() string {
	root := ""
	if api.rt != nil {
		root = api.rt.ShellRoot()
	}

	if strings.TrimSpace(root) == "" {
		cwd, err := os.Getwd()
		if err == nil {
			return cwd
		}
		return root
	}

	abs, err := filepath.Abs(root)
	if err != nil {
		return root
	}
	return abs
}
