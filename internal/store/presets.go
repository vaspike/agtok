package store

import (
    "encoding/json"
    "errors"
    "fmt"
    "io/fs"
    "os"
    "path/filepath"
    core "tks/internal/core"
    "tks/internal/fsx"
)

type presetFile struct {
    Version int            `json:"version"`
    Presets []core.Preset  `json:"presets"`
}

func configDir() string {
    if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
        return filepath.Join(d, "token-switcher", "presets")
    }
    h, _ := os.UserHomeDir()
    if h == "" { h = "." }
    return filepath.Join(h, ".config", "token-switcher", "presets")
}

// PresetsDir returns the directory where agent preset JSON files are stored.
func PresetsDir() string {
    return configDir()
}

func pathFor(agent core.AgentID) string {
    return filepath.Join(configDir(), fmt.Sprintf("%s.json", string(agent)))
}

// LoadPresets returns all presets for an agent.
func LoadPresets(agent core.AgentID) ([]core.Preset, error) {
    p := pathFor(agent)
    b, err := os.ReadFile(p)
    if err != nil {
        if errors.Is(err, os.ErrNotExist) { return nil, nil }
        return nil, err
    }
    var f presetFile
    if err := json.Unmarshal(b, &f); err != nil { return nil, err }
    return f.Presets, nil
}

// AddPreset appends a preset; alias must be unique within agent.
func AddPreset(agent core.AgentID, pr core.Preset) error {
    list, _ := LoadPresets(agent)
    for _, p := range list {
        if p.Alias == pr.Alias {
            return fmt.Errorf("alias already exists: %s", pr.Alias)
        }
    }
    list = append(list, pr)
    pf := presetFile{Version: 1, Presets: list}
    data, _ := json.MarshalIndent(&pf, "", "  ")
    path := pathFor(agent)
    if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil { return err }
    return fsx.AtomicWrite(path, data, fs.FileMode(0o600))
}

// GetPreset finds a preset by alias.
func GetPreset(agent core.AgentID, alias string) (core.Preset, error) {
    list, _ := LoadPresets(agent)
    for _, p := range list {
        if p.Alias == alias { return p, nil }
    }
    return core.Preset{}, fmt.Errorf("preset not found: %s", alias)
}

// RemovePreset deletes a preset by alias and writes back atomically.
func RemovePreset(agent core.AgentID, alias string) error {
    list, err := LoadPresets(agent)
    if err != nil { return err }
    kept := make([]core.Preset, 0, len(list))
    removed := false
    for _, p := range list {
        if p.Alias == alias { removed = true; continue }
        kept = append(kept, p)
    }
    if !removed { return fmt.Errorf("preset not found: %s", alias) }
    pf := presetFile{Version: 1, Presets: kept}
    data, _ := json.MarshalIndent(&pf, "", "  ")
    path := pathFor(agent)
    if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil { return err }
    return fsx.AtomicWrite(path, data, fs.FileMode(0o600))
}

// RenamePreset renames a preset alias, ensuring uniqueness within the agent.
func RenamePreset(agent core.AgentID, oldAlias, newAlias string) error {
    list, err := LoadPresets(agent)
    if err != nil { return err }
    found := -1
    for i, p := range list {
        if p.Alias == oldAlias { found = i }
        if p.Alias == newAlias { return fmt.Errorf("alias already exists: %s", newAlias) }
    }
    if found < 0 { return fmt.Errorf("preset not found: %s", oldAlias) }
    list[found].Alias = newAlias
    pf := presetFile{Version: 1, Presets: list}
    data, _ := json.MarshalIndent(&pf, "", "  ")
    path := pathFor(agent)
    if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil { return err }
    return fsx.AtomicWrite(path, data, fs.FileMode(0o600))
}

// UpdatePreset updates fields of a preset. url/token/model are optional via pointers.
// clearToken/clearModel indicate explicit clearing.
func UpdatePreset(agent core.AgentID, oldAlias, newAlias string, url *string, token *string, model *string, clearToken bool, clearModel bool) error {
    list, err := LoadPresets(agent)
    if err != nil { return err }
    idx := -1
    for i, p := range list {
        if p.Alias == oldAlias { idx = i }
        if newAlias != oldAlias && p.Alias == newAlias {
            return fmt.Errorf("alias already exists: %s", newAlias)
        }
    }
    if idx < 0 { return fmt.Errorf("preset not found: %s", oldAlias) }
    // alias
    if newAlias != "" { list[idx].Alias = newAlias }
    // url
    if url != nil { list[idx].URL = *url }
    // token (three-state)
    if clearToken { list[idx].Token = "" } else if token != nil { list[idx].Token = *token }
    // model (three-state), only meaningful for Claude but harmless elsewhere
    if clearModel { list[idx].Model = "" } else if model != nil { list[idx].Model = *model }
    pf := presetFile{Version: 1, Presets: list}
    data, _ := json.MarshalIndent(&pf, "", "  ")
    path := pathFor(agent)
    if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil { return err }
    return fsx.AtomicWrite(path, data, fs.FileMode(0o600))
}
