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

