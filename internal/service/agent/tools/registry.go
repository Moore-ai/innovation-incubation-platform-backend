package tools

import (
	"encoding/json"
	"fmt"
)

type ToolRegistry struct {
	tools map[string]Tool
}

func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{tools: make(map[string]Tool)}
}

func (r *ToolRegistry) Register(t Tool) {
	r.tools[t.Name()] = t
}

func (r *ToolRegistry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

func (r *ToolRegistry) ListForRole(role string) []Tool {
	var out []Tool
	for _, t := range r.tools {
		for _, allowed := range t.AllowedRoles() {
			if allowed == role {
				out = append(out, t)
				break
			}
		}
	}
	return out
}

func (r *ToolRegistry) All() []Tool {
	var out []Tool
	for _, t := range r.tools {
		out = append(out, t)
	}
	return out
}

func (r *ToolRegistry) GetOutputSchema(name string) (json.RawMessage, error) {
	t, ok := r.tools[name]
	if !ok {
		return nil, fmt.Errorf("tool %s not found", name)
	}
	return t.OutputSchema(), nil
}
