package ui

import (
    "context"
    "os/exec"
    "regexp"
    "strings"
    "time"

    core "tks/internal/core"
)

var verRe = regexp.MustCompile(`(?i)\bv?\d+\.\d+(?:\.\d+)*(?:-[0-9A-Za-z.\-]+)?`)

// detectVersion runs the agent binary with a version flag and parses the output.
// Returns (text, installed). If not installed, text is "Not installed".
// If installed but cannot parse, text is "Unknown".
func detectVersion(id core.AgentID) (string, bool) {
    var bin string
    var args []string
    switch id {
    case core.AgentClaude:
        bin = "claude"; args = []string{"-v"}
    case core.AgentGemini:
        bin = "gemini"; args = []string{"-v"}
    case core.AgentCodex:
        bin = "codex"; args = []string{"-V"}
    default:
        return "Unknown", false
    }
    if _, err := exec.LookPath(bin); err != nil {
        return "Not installed", false
    }
    ctx, cancel := context.WithTimeout(context.Background(), 1200*time.Millisecond)
    defer cancel()
    out, err := exec.CommandContext(ctx, bin, args...).CombinedOutput()
    if err != nil && ctx.Err() == context.DeadlineExceeded {
        return "Unknown", true
    }
    s := strings.TrimSpace(string(out))
    if s == "" { return "Unknown", true }
    m := verRe.FindString(s)
    if m == "" { return "Unknown", true }
    // keep original (do not strip leading 'v') per requirement
    return m, true
}
