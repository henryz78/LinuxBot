package tool

import (
	"context"
	"fmt"
)

type Tool interface {
	Name() string
	Execute(ctx context.Context, req ToolRequest) (ToolResult, error)
}

type ToolRequest struct {
	Name  string
	Input map[string]string
}

type ApprovalDecision string

const (
	ApprovalApprove ApprovalDecision = "approve"
	ApprovalReject  ApprovalDecision = "reject"
	ApprovalAlways  ApprovalDecision = "always"
)

type ApprovalRequest struct {
	Command string
	Reason  string
}

type ApprovalFunc func(ctx context.Context, req ApprovalRequest) (ApprovalDecision, error)

type ToolResult struct {
	Status              string
	Output              string
	Stdout              string
	Stderr              string
	ErrorText           string
	ExitCode            int
	DurationMillis      int64
	StdoutBytesObserved int64
	StderrBytesObserved int64
	Command             string
	ApprovalDecision    ApprovalDecision
}

type Router struct {
	tools map[string]Tool
}

func NewRouter(tools ...Tool) *Router {
	router := &Router{tools: map[string]Tool{}}
	for _, item := range tools {
		router.tools[item.Name()] = item
	}
	return router
}

func (r *Router) Execute(ctx context.Context, req ToolRequest) (ToolResult, error) {
	item, ok := r.tools[req.Name]
	if !ok {
		return ToolResult{}, fmt.Errorf("unknown tool %q", req.Name)
	}
	return item.Execute(ctx, req)
}
