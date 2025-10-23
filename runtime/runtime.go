package runtime

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/openai/openai-go"
	"github.com/vanclief/agent-composer/models/agent"
	"github.com/vanclief/agent-composer/server/controller"
	"github.com/vanclief/compose/components/scheduler"
	"github.com/vanclief/compose/drivers/databases/relational"
	"github.com/vanclief/ez"
)

type Runtime struct {
	rootCtx   context.Context
	db        *relational.DB
	scheduler *scheduler.Scheduler
	openai    *openai.Client
}

type hookSub struct {
	cancel      context.CancelFunc
	unsubscribe func() error
}

func New(rootCtx context.Context, ctrl *controller.Controller, sch *scheduler.Scheduler) (*Runtime, error) {
	const op = "runtime.New"

	if ctrl == nil {
		return nil, ez.Root(op, ez.EINTERNAL, "Controller reference is nil")
	}

	// TODO: Should dinamically chose which LLM Providers to initialize
	// for now, only OpenAI is supported

	rt := &Runtime{
		rootCtx:   rootCtx,
		db:        ctrl.DB,
		scheduler: sch,
	}

	err := rt.SetOpenAIClient()
	if err != nil {
		return nil, ez.Wrap(op, err)
	}

	return rt, nil
}

func (rt *Runtime) SetOpenAIClient() error {
	const op = "runtime.SetOpenAIClient"

	apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if apiKey == "" {
		return ez.New(op, ez.EINVALID, "missing env var OPENAI_API_KEY", nil)
	}

	client := openai.NewClient()

	rt.openai = &client

	return nil
}

func (rt *Runtime) ValidateModel(ctx context.Context, provider agent.LLMProvider, model string) error {
	const op = "ChatGPT.ValidateModel"

	if model == "" {
		return ez.New(op, ez.EINVALID, "model is required", nil)
	}

	// Uses the official SDK's Models service (Get) to verify the model ID.
	// Any 4xx/5xx from the API bubbles up here.
	_, err := rt.openai.Models.Get(ctx, model)
	if err != nil {
		errMsg := fmt.Sprintf("ChatGPT model %s does not exist", model)
		return ez.New(op, ez.EINVALID, errMsg, err)
	}

	return nil
}
