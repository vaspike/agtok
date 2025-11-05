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
    verinfo "tks/internal/version"
)

type presetFile struct {
    Version int            `json:"version"`
    ConfigVersion string   `json:"config_version,omitempty"`
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
    f, err := loadPresetFile(agent)
    if err != nil { return nil, err }
    return f.Presets, nil
}

// loadPresetFile reads the full preset file including metadata.
func loadPresetFile(agent core.AgentID) (presetFile, error) {
    p := pathFor(agent)
    b, err := os.ReadFile(p)
    if err != nil {
        if errors.Is(err, os.ErrNotExist) { return presetFile{Version: 1}, nil }
        return presetFile{}, err
    }
    var f presetFile
    if err := json.Unmarshal(b, &f); err != nil { return presetFile{}, err }
    if f.Version == 0 { f.Version = 1 }
    return f, nil
}

// writePresetFile writes the full preset file and stamps config_version.
func writePresetFile(agent core.AgentID, f presetFile) error {
    if f.Version == 0 { f.Version = 1 }
    f.ConfigVersion = verinfo.Version
    data, _ := json.MarshalIndent(&f, "", "  ")
    path := pathFor(agent)
    if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil { return err }
    return fsx.AtomicWrite(path, data, fs.FileMode(0o600))
}

// AddPreset appends a preset; alias must be unique within agent.
func AddPreset(agent core.AgentID, pr core.Preset) error {
    f, _ := loadPresetFile(agent)
    list := f.Presets
    for _, p := range list {
        if p.Alias == pr.Alias {
            return fmt.Errorf("alias already exists: %s", pr.Alias)
        }
    }
    list = append(list, pr)
    f.Presets = list
    return writePresetFile(agent, f)
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
    f, err := loadPresetFile(agent)
    if err != nil { return err }
    list := f.Presets
    kept := make([]core.Preset, 0, len(list))
    removed := false
    for _, p := range list {
        if p.Alias == alias { removed = true; continue }
        kept = append(kept, p)
    }
    if !removed { return fmt.Errorf("preset not found: %s", alias) }
    f.Presets = kept
    return writePresetFile(agent, f)
}

// RenamePreset renames a preset alias, ensuring uniqueness within the agent.
func RenamePreset(agent core.AgentID, oldAlias, newAlias string) error {
    f, err := loadPresetFile(agent)
    if err != nil { return err }
    list := f.Presets
    found := -1
    for i, p := range list {
        if p.Alias == oldAlias { found = i }
        if p.Alias == newAlias { return fmt.Errorf("alias already exists: %s", newAlias) }
    }
    if found < 0 { return fmt.Errorf("preset not found: %s", oldAlias) }
    list[found].Alias = newAlias
    f.Presets = list
    return writePresetFile(agent, f)
}

// UpdatePreset updates fields of a preset. url/token/model are optional via pointers.
// clearToken/clearModel indicate explicit clearing.
func UpdatePreset(agent core.AgentID, oldAlias, newAlias string, url *string, token *string, model *string, clearToken bool, clearModel bool) error {
    f, err := loadPresetFile(agent)
    if err != nil { return err }
    list := f.Presets
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
    f.Presets = list
    return writePresetFile(agent, f)
}

// MigrateOnInit backfills missing model for Gemini/Codex when schema version==1,
// and updates config_version to current. Claude is skipped for backfill.
func MigrateOnInit(agent core.AgentID, diskModel string) error {
    f, err := loadPresetFile(agent)
    if err != nil { return err }
    if f.Version != 1 {
        // still stamp config version on init
        return writePresetFile(agent, f)
    }
    if (agent == core.AgentGemini || agent == core.AgentCodex) && diskModel != "" {
        for i := range f.Presets {
            if f.Presets[i].Model == "" { f.Presets[i].Model = diskModel }
        }
    }
    // write back (also stamps config_version)
    return writePresetFile(agent, f)
}
