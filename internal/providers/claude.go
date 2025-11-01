package providers

import (
    "context"
    "encoding/json"
    "errors"
    "io/fs"
    "os"
    "path/filepath"
    core "tks/internal/core"
    "tks/internal/fsx"
)

type claude struct{}

func (c *claude) ID() core.AgentID { return core.AgentClaude }

func (c *claude) Paths() []string {
    return []string{joinHome(".claude", "settings.json")}
}

type claudeSettings struct {
    Env map[string]string `json:"env"`
}

func (c *claude) Read(ctx context.Context) (core.Fields, error) {
    p := c.Paths()[0]
    b, err := os.ReadFile(p)
    if err != nil {
        if errors.Is(err, os.ErrNotExist) {
            return core.Fields{}, nil
        }
        return core.Fields{}, err
    }
    var s claudeSettings
    if err := json.Unmarshal(b, &s); err != nil {
        return core.Fields{}, err
    }
    if s.Env == nil {
        s.Env = map[string]string{}
    }
    // Prefer common keys in order: AUTH_TOKEN -> API_TOKEN -> API_KEY
    token := s.Env["ANTHROPIC_AUTH_TOKEN"]
    if token == "" {
        token = s.Env["ANTHROPIC_API_TOKEN"]
    }
    if token == "" {
        token = s.Env["ANTHROPIC_API_KEY"]
    }
    return core.Fields{
        URL:   s.Env["ANTHROPIC_BASE_URL"],
        Token: token,
    }, nil
}

func (c *claude) Write(ctx context.Context, fields core.Fields) (core.Backup, error) {
    p := c.Paths()[0]
    _ = os.MkdirAll(filepath.Dir(p), 0o700)
    s := claudeSettings{Env: map[string]string{}}
    if b, err := os.ReadFile(p); err == nil {
        _ = json.Unmarshal(b, &s)
        if s.Env == nil { s.Env = map[string]string{} }
    }
    s.Env["ANTHROPIC_BASE_URL"] = fields.URL
    // allow empty token to keep existing; when provided, write only AUTH_TOKEN as requested
    if fields.Token != "" {
        s.Env["ANTHROPIC_AUTH_TOKEN"] = fields.Token
    }
    out, err := json.MarshalIndent(&s, "", "  ")
    if err != nil { return core.Backup{}, err }
    if err := fsx.BackupFile(p); err != nil && !errors.Is(err, os.ErrNotExist) {
        return core.Backup{}, err
    }
    if err := fsx.AtomicWrite(p, out, fs.FileMode(0o600)); err != nil {
        return core.Backup{}, err
    }
    return core.Backup{}, nil
}

func (c *claude) Validate(f core.Fields) error { return core.ValidateFields(f) }
