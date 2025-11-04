package providers

import (
    "context"
    core "tks/internal/core"
)

type Provider interface {
    ID() core.AgentID
    Paths() []string
    Read(ctx context.Context) (core.Fields, error)
    Write(ctx context.Context, fields core.Fields) (core.Backup, error)
    Validate(fields core.Fields) error
}

// Context keys for provider-specific controls
type ctxKey string

// CtxKeyClaudeClearModel: when ctx has this key set to true in Write,
// Claude provider will remove ANTHROPIC_MODEL from settings.json
var CtxKeyClaudeClearModel ctxKey = "claude_clear_model"
