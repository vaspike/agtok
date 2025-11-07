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
    // read base_url from toml; provider can be model_providers.*; prefer codex, fallback to first found
    var url string
    var model string
    if f, err := os.Open(paths[0]); err == nil {
        defer f.Close()
        s := bufio.NewScanner(f)
        inProviders := false
        curHeader := ""
        // preserve encounter order
        providerOrder := []string{}
        urlByProvider := map[string]string{}
        selProvider := "" // from root-level model_provider
        for s.Scan() {
            line := strings.TrimSpace(s.Text())
            if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
                inProviders = false
                curHeader = ""
                // detect provider section: [model_providers.xxx]
                if strings.HasPrefix(line, "[model_providers.") && strings.HasSuffix(line, "]") {
                    inProviders = true
                    curHeader = line
                    providerOrder = append(providerOrder, line)
                }
                continue
            }
            if line == "" || strings.HasPrefix(line, "#") { continue }
            if i := strings.Index(line, "="); i >= 0 {
                k := strings.TrimSpace(line[:i])
                v := strings.Trim(strings.TrimSpace(line[i+1:]), "\"'")
                if inProviders {
                    if k == "base_url" { urlByProvider[curHeader] = v }
                } else {
                    if k == "model" { model = v }
                    if k == "model_provider" { selProvider = v }
                }
            }
        }
        // choose url: prefer selected provider, else codex, else first provider
        if selProvider != "" {
            if v, ok := urlByProvider["[model_providers."+selProvider+"]"]; ok && v != "" { url = v }
        }
        if url == "" {
            if v, ok := urlByProvider["[model_providers.codex]"]; ok && v != "" { url = v }
        }
        if url == "" {
            for _, h := range providerOrder { if v := urlByProvider[h]; v != "" { url = v; break } }
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
    return core.Fields{URL: url, Token: token, Model: model}, nil
}

func (c *codex) Write(ctx context.Context, fields core.Fields) (core.Backup, error) {
    paths := c.Paths()
    // ensure dir
    _ = os.MkdirAll(filepath.Dir(paths[0]), 0o700)
    // update toml
    // naive update: replace/ensure target provider section's base_url; root-level 'model'
    var lines []string
    var hadTargetSection bool
    var wroteKey bool
    var sawModel bool
    if b, err := os.ReadFile(paths[0]); err == nil {
        for _, ln := range strings.Split(string(b), "\n") {
            lines = append(lines, ln)
        }
    }
    var out []string
    // pre-scan to choose target provider header for writing
    providerOrder := []string{}
    hasCodex := false
    selProvider := ""
    inAny := false
    for _, ln := range lines {
        line := strings.TrimSpace(ln)
        if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
            inAny = true
            if line == "[model_providers.codex]" { hasCodex = true }
            if strings.HasPrefix(line, "[model_providers.") && strings.HasSuffix(line, "]") {
                providerOrder = append(providerOrder, line)
            }
            continue
        }
        if line == "" || strings.HasPrefix(line, "#") { continue }
        if !inAny {
            if i := strings.Index(line, "="); i >= 0 {
                k := strings.TrimSpace(line[:i])
                v := strings.Trim(strings.TrimSpace(line[i+1:]), "\"'")
                if k == "model_provider" { selProvider = v }
            }
        }
    }
    targetHeader := "[model_providers.codex]"
    if selProvider != "" {
        targetHeader = "[model_providers." + selProvider + "]"
    } else if !hasCodex && len(providerOrder) > 0 {
        targetHeader = providerOrder[0]
    }
    inSection := false
    for _, ln := range lines {
        line := strings.TrimSpace(ln)
        if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
            if inSection && !wroteKey {
                out = append(out, "base_url = \""+fields.URL+"\"")
                wroteKey = true
            }
            inSection = (line == targetHeader)
            if inSection { hadTargetSection = true }
            out = append(out, ln)
            continue
        }
        if inSection {
            if strings.HasPrefix(line, "base_url") {
                out = append(out, "base_url = \""+fields.URL+"\"")
                wroteKey = true
                continue
            }
        } else {
            // root level keys: parse k=v to match exact "model"
            if i := strings.Index(line, "="); i >= 0 {
                k := strings.TrimSpace(line[:i])
                if k == "model" {
                    sawModel = true
                    if v, ok := ctx.Value(CtxKeyCodexClearModel).(bool); ok && v {
                        // drop this line (clear model)
                        continue
                    }
                    if fields.Model != "" {
                        out = append(out, "model = \""+fields.Model+"\"")
                        continue
                    }
                    // keep original if not clearing or setting
                }
            }
        }
        out = append(out, ln)
    }
    if !hadTargetSection {
        // no target section exists; create it at EOF (prefer codex name or fallback)
        out = append(out, targetHeader)
        out = append(out, "base_url = \""+fields.URL+"\"")
        wroteKey = true
    } else if inSection && !wroteKey {
        out = append(out, "base_url = \""+fields.URL+"\"")
    }
    // ensure model root-level line if needed
    if !sawModel {
        if v, ok := ctx.Value(CtxKeyCodexClearModel).(bool); ok && v {
            // nothing to add (cleared)
        } else if fields.Model != "" {
            out = append([]string{"model = \""+fields.Model+"\""}, out...)
        }
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
