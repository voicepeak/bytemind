package tools

import (
	"context"
	"encoding/json"
)

type workerRunRequest struct {
	Resolved  ResolvedTool
	RawArgs   json.RawMessage
	Execution *ExecutionContext
}

type executorWorker interface {
	Run(context.Context, workerRunRequest) (string, error)
}

type inProcessWorker struct {
	normalizer OutputNormalizer
}

func (w inProcessWorker) Run(ctx context.Context, req workerRunRequest) (string, error) {
	normalizer := w.normalizer
	if normalizer == nil {
		normalizer = maxCharsOutputNormalizer{}
	}
	runCtx, cancel := context.WithTimeout(ctx, executionTimeout(req.RawArgs, req.Resolved.Spec))
	defer cancel()

	output, err := req.Resolved.Tool.Run(runCtx, req.RawArgs, req.Execution)
	if err != nil {
		return "", normalizeToolError(err)
	}
	return normalizer.Normalize(output, req.Resolved), nil
}

func shouldRouteToWorker(toolName string, execCtx *ExecutionContext) bool {
	if execCtx == nil || !execCtx.SandboxEnabled {
		return false
	}
	switch toolName {
	case "run_shell", "read_file", "write_file":
		return true
	default:
		return false
	}
}
