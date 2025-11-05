package providers

import (
    "bufio"
    "context"
    "errors"
    "io/fs"
    "os"
    "path/filepath"
    "strings"
    core "tks/internal/core"
    "tks/internal/fsx"
)

type gemini struct{}

func (g *gemini) ID() core.AgentID { return core.AgentGemini }

func (g *gemini) Paths() []string { return []string{joinHome(".gemini", ".env")} }

func (g *gemini) Read(ctx context.Context) (core.Fields, error) {
    p := g.Paths()[0]
    f, err := os.Open(p)
    if err != nil {
        if errors.Is(err, os.ErrNotExist) { return core.Fields{}, nil }
        return core.Fields{}, err
    }
    defer f.Close()
    var url, token, model string
    s := bufio.NewScanner(f)
    for s.Scan() {
        line := strings.TrimSpace(s.Text())
        if line == "" || strings.HasPrefix(line, "#") { continue }
        if i := strings.IndexByte(line, '='); i >= 0 {
            k := strings.TrimSpace(line[:i])
            v := strings.TrimSpace(line[i+1:])
            if k == "GOOGLE_GEMINI_BASE_URL" { url = v }
            if k == "GEMINI_API_KEY" { token = v }
            if k == "GEMINI_MODEL" { model = v }
        }
    }
    return core.Fields{URL: url, Token: token, Model: model}, nil
}

func (g *gemini) Write(ctx context.Context, fields core.Fields) (core.Backup, error) {
    p := g.Paths()[0]
    _ = os.MkdirAll(filepath.Dir(p), 0o700)
    content := make(map[string]string)
    // read existing
    if b, err := os.ReadFile(p); err == nil {
        for _, line := range strings.Split(string(b), "\n") {
            line = strings.TrimSpace(line)
            if line == "" || strings.HasPrefix(line, "#") { continue }
            if i := strings.IndexByte(line, '='); i >= 0 {
                k := strings.TrimSpace(line[:i])
                v := strings.TrimSpace(line[i+1:])
                content[k] = v
            }
        }
    }
    if fields.URL != "" { content["GOOGLE_GEMINI_BASE_URL"] = fields.URL }
    if fields.Token != "" { content["GEMINI_API_KEY"] = fields.Token }
    // model write/clear per context
    if v, ok := ctx.Value(CtxKeyGeminiClearModel).(bool); ok && v {
        delete(content, "GEMINI_MODEL")
    } else if fields.Model != "" {
        content["GEMINI_MODEL"] = fields.Model
    }
    // rebuild .env (MVP: no comments preserved)
    var b strings.Builder
    keys := []string{"GOOGLE_GEMINI_BASE_URL", "GEMINI_API_KEY", "GEMINI_MODEL"}
    for k, v := range content {
        // include any extra keys as well
        if k != keys[0] && k != keys[1] && k != keys[2] {
            b.WriteString(k + "=" + v + "\n")
        }
    }
    // write our keys last in fixed order
    if v, ok := content[keys[0]]; ok { b.WriteString(keys[0] + "=" + v + "\n") }
    if v, ok := content[keys[1]]; ok { b.WriteString(keys[1] + "=" + v + "\n") }
    if v, ok := content[keys[2]]; ok { b.WriteString(keys[2] + "=" + v + "\n") }
    if err := fsx.BackupFile(p); err != nil && !errors.Is(err, os.ErrNotExist) { return core.Backup{}, err }
    if err := fsx.AtomicWrite(p, []byte(b.String()), fs.FileMode(0o600)); err != nil { return core.Backup{}, err }
    return core.Backup{}, nil
}

func (g *gemini) Validate(f core.Fields) error { return core.ValidateFields(f) }
