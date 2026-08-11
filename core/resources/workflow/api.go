package workflow

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/uptrace/bun"
	"github.com/vanclief/agent-composer/core/controller"
	"github.com/vanclief/agent-composer/core/resources/workflow/conversations"
	"github.com/vanclief/agent-composer/core/resources/workflow/executions"
	"github.com/vanclief/agent-composer/core/resources/workflow/nodeexecutions"
	"github.com/vanclief/agent-composer/core/resources/workflow/worktrees"
	"github.com/vanclief/agent-composer/runtime"
	workflowruntime "github.com/vanclief/agent-composer/workflow"
	"github.com/vanclief/agent-composer/worktree"
)

type API struct {
	Executions     *executions.API
	NodeExecutions *nodeexecutions.API
	Conversations  *conversations.API
	Worktrees      *worktrees.API
	Registry       *workflowruntime.Registry
	rt             *runtime.Runtime
	db             bun.IDB
}

func NewAPI(ctrl *controller.Controller, rt *runtime.Runtime) *API {
	if ctrl == nil {
		panic("Controller reference is nil")
	}

	registry := workflowruntime.NewRegistry(ctrl.DB)
	manager := &worktree.Manager{Root: worktree.DefaultRoot()}
	executionsAPI := executions.NewAPI(ctrl, rt, manager, registry)
	nodeExecutionsAPI := nodeexecutions.NewAPI(ctrl)
	conversationsAPI := conversations.NewAPI(ctrl)
	worktreesAPI := worktrees.NewAPI(manager)

	return &API{
		Executions:     executionsAPI,
		NodeExecutions: nodeExecutionsAPI,
		Conversations:  conversationsAPI,
		Worktrees:      worktreesAPI,
		Registry:       registry,
		rt:             rt,
		db:             ctrl.DB,
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
