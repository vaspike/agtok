package core

import "time"

type AgentID string

const (
    AgentClaude AgentID = "claude"
    AgentGemini AgentID = "gemini"
    AgentCodex  AgentID = "codex"
)

// Fields are the two values we manage per agent.
type Fields struct {
    URL   string
    Token string
}

// DiskState holds actual values read from disk.
type DiskState struct {
    Agent  AgentID
    Fields Fields
    Status string   // OK | MissingFile | MissingKey | Error
    Paths  []string // files involved
}

// Preset is stored per agent, alias unique within agent.
type Preset struct {
    Alias   string `json:"alias"`
    URL     string `json:"url"`
    Token   string `json:"token"`
    AddedAt string `json:"added_at"` // UI does not display this
}

// Backup info for write operations.
type Backup struct {
    Files map[string]string // oldPath -> backupPath (reserved for future)
    Time  time.Time
}

// Diff renders a simple diff between old and new values.
func Diff(old, new Fields) string {
    mask := func(s string) string {
        if len(s) <= 4 {
            return "****"
        }
        return "****" + s[len(s)-4:]
    }
    out := "Diff (URL, Token):\n"
    if old.URL != new.URL {
        out += "  URL:  " + old.URL + " -> " + new.URL + "\n"
    } else {
        out += "  URL:  (no change)\n"
    }
    if old.Token != new.Token {
        out += "  Token:" + mask(old.Token) + " -> " + mask(new.Token) + "\n"
    } else {
        out += "  Token: (no change)\n"
    }
    return out
}
