package agent

import (
	"context"

	"github.com/stelmakhdigital/ai"
	coreagent "stell/agent"
)

// CoreLoop собирает stell/agent.Loop, связанный с этим product Agent.
// runOnce заполняет product-колбэки через wireProductLoop перед RunPrepared.
func (a *Agent) CoreLoop() *coreagent.Loop {
	mode := coreagent.ToolExecutionParallel
	maxIter := 0
	if a.Config != nil {
		if a.Config.Settings.ToolExecutionMode() == "sequential" {
			mode = coreagent.ToolExecutionSequential
		}
		maxIter = a.Config.Settings.MaxToolIterations()
	}
	mid := a.Model.Model
	if mid == "" {
		mid = a.Model.Name
	}
	return &coreagent.Loop{
		Registry:      a.Registry,
		Tools:         a.Tools,
		Sessions:      a.Sessions,
		ModelName:     a.Model.Name,
		ModelID:       mid,
		MaxIterations: maxIter,
		ToolExecution: mode,
		BuildSystem: func(ctx context.Context) string {
			return a.buildSystem(ctx, "")
		},
		ConvertMessages: coreagent.ConvertToLlm,
		SteerFn: func() (string, bool) {
			if a.SteerFn == nil {
				return "", false
			}
			msg, _, ok := a.SteerFn()
			return msg, ok
		},
		FollowUpMessage: func() (ai.Message, bool) {
			if a.FollowUpFn == nil {
				return ai.Message{}, false
			}
			msg, imgs, ok := a.FollowUpFn()
			if !ok || (msg == "" && len(imgs) == 0) {
				return ai.Message{}, false
			}
			msg = PromptWithImages(msg, imgs)
			return BuildUserMessage(msg, imgs, a.Model), true
		},
		BeforeToolCall: func(ctx context.Context, call ai.ToolCall) (bool, map[string]any) {
			return a.emitToolCallHook(ctx, call)
		},
	}
}
