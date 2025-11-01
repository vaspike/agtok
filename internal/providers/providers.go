package providers

import (
    core "tks/internal/core"
)

// NewProvider returns a concrete provider for an agent.
func NewProvider(id core.AgentID) Provider {
    switch id {
    case core.AgentClaude:
        return &claude{}
    case core.AgentGemini:
        return &gemini{}
    case core.AgentCodex:
        return &codex{}
    default:
        return nil
    }
}

