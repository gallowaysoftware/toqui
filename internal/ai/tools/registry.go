package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gallowaysoftware/toqui-backend/internal/ai"
)

type Tool interface {
	Definition() ai.ToolDefinition
	Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error)
}

// Registry holds the set of available AI tools. It is safe for concurrent
// reads (Get, Execute, Definitions) but NOT concurrent writes. All Register
// calls must happen during initialization, before the server starts accepting
// requests. Per-request extra tools use a separate map in chat/service.go.
type Registry struct {
	tools map[string]Tool
}

func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

func (r *Registry) Register(tool Tool) {
	r.tools[tool.Definition().Name] = tool
}

func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

func (r *Registry) Definitions() []ai.ToolDefinition {
	defs := make([]ai.ToolDefinition, 0, len(r.tools))
	for _, t := range r.tools {
		defs = append(defs, t.Definition())
	}
	return defs
}

func (r *Registry) Execute(ctx context.Context, name string, args json.RawMessage) (json.RawMessage, error) {
	tool, ok := r.tools[name]
	if !ok {
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
	return tool.Execute(ctx, args)
}
