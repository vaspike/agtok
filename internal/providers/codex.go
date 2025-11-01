package providers

import (
    "bufio"
    "context"
    "encoding/json"
    "errors"
    "io/fs"
    "os"
    "path/filepath"
    "strings"
    core "tks/internal/core"
    "tks/internal/fsx"
)

type codex struct{}

func (c *codex) ID() core.AgentID { return core.AgentCodex }

func (c *codex) Paths() []string {
    return []string{joinHome(".codex", "config.toml"), joinHome(".codex", "auth.json")}
}

func (c *codex) Read(ctx context.Context) (core.Fields, error) {
    paths := c.Paths()
    // read base_url from toml
    var url string
    if f, err := os.Open(paths[0]); err == nil {
        defer f.Close()
        s := bufio.NewScanner(f)
        inSection := false
        for s.Scan() {
            line := strings.TrimSpace(s.Text())
            if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
                inSection = (line == "[model_providers.codex]")
                continue
            }
            if !inSection || line == "" || strings.HasPrefix(line, "#") { continue }
            if i := strings.Index(line, "="); i >= 0 {
                k := strings.TrimSpace(line[:i])
                v := strings.Trim(strings.TrimSpace(line[i+1:]), "\"'")
                if k == "base_url" { url = v }
            }
        }
    }
    // read token from auth.json
    var token string
    if b, err := os.ReadFile(paths[1]); err == nil {
        var m map[string]string
        if json.Unmarshal(b, &m) == nil {
            token = m["OPENAI_API_KEY"]
        }
    }
    return core.Fields{URL: url, Token: token}, nil
}

func (c *codex) Write(ctx context.Context, fields core.Fields) (core.Backup, error) {
    paths := c.Paths()
    // ensure dir
    _ = os.MkdirAll(filepath.Dir(paths[0]), 0o700)
    // update toml
    // naive update: replace/ensure section and base_url line
    var lines []string
    var hadSection bool
    var wroteKey bool
    if b, err := os.ReadFile(paths[0]); err == nil {
        for _, ln := range strings.Split(string(b), "\n") {
            lines = append(lines, ln)
        }
    }
    var out []string
    inSection := false
    for _, ln := range lines {
        line := strings.TrimSpace(ln)
        if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
            if inSection && !wroteKey {
                out = append(out, "base_url = \""+fields.URL+"\"")
                wroteKey = true
            }
            inSection = (line == "[model_providers.codex]")
            if inSection { hadSection = true }
            out = append(out, ln)
            continue
        }
        if inSection {
            if strings.HasPrefix(line, "base_url") {
                out = append(out, "base_url = \""+fields.URL+"\"")
                wroteKey = true
                continue
            }
        }
        out = append(out, ln)
    }
    if !hadSection {
        out = append(out, "[model_providers.codex]")
        out = append(out, "base_url = \""+fields.URL+"\"")
        wroteKey = true
    } else if inSection && !wroteKey {
        out = append(out, "base_url = \""+fields.URL+"\"")
    }
    tomlOut := strings.Join(out, "\n")
    if err := fsx.BackupFile(paths[0]); err != nil && !errors.Is(err, os.ErrNotExist) { return core.Backup{}, err }
    if err := fsx.AtomicWrite(paths[0], []byte(tomlOut), fs.FileMode(0o600)); err != nil { return core.Backup{}, err }

    // update auth.json
    auth := map[string]string{}
    if b, err := os.ReadFile(paths[1]); err == nil {
        _ = json.Unmarshal(b, &auth)
    }
    if fields.Token != "" {
        auth["OPENAI_API_KEY"] = fields.Token
    }
    jb, _ := json.MarshalIndent(auth, "", "  ")
    if err := fsx.BackupFile(paths[1]); err != nil && !errors.Is(err, os.ErrNotExist) { return core.Backup{}, err }
    if err := fsx.AtomicWrite(paths[1], jb, fs.FileMode(0o600)); err != nil { return core.Backup{}, err }
    return core.Backup{}, nil
}

func (c *codex) Validate(f core.Fields) error { return core.ValidateFields(f) }

