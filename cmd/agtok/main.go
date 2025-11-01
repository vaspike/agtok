package main

import (
    "context"
    "flag"
    "fmt"
    "os"
    "strings"
    "time"

    core "tks/internal/core"
    "tks/internal/providers"
    "tks/internal/store"
    "tks/internal/util"
    ui "tks/internal/ui"
)

func usage() {
    fmt.Fprintf(os.Stderr, "agtok - AI agent token control\n\n")
    fmt.Fprintf(os.Stderr, "Usage:\n")
    fmt.Fprintf(os.Stderr, "  agtok list --agent <claude|gemini|codex>\n")
    fmt.Fprintf(os.Stderr, "  agtok presets list --agent <id>\n")
    fmt.Fprintf(os.Stderr, "  agtok presets add --agent <id> [--alias <name>] --url <u> [--token <t>]\n")
    fmt.Fprintf(os.Stderr, "  agtok apply --agent <id> --alias <name> [--dry-run]\n")
    fmt.Fprintf(os.Stderr, "  agtok apply --agent <id> --url <u> [--token <t>] [--dry-run]\n")
    fmt.Fprintf(os.Stderr, "  agtok init [--agent <id>] [--alias <name>]\n")
}

func main() {
    // Default: TUI when no args
    if len(os.Args) == 1 {
        if err := ui.Run(); err != nil { fmt.Println(err); os.Exit(1) }
        return
    }

    cmd := os.Args[1]
    switch cmd {
    case "list":
        listCmd(os.Args[2:])
    case "presets":
        presetsCmd(os.Args[2:])
    case "apply":
        applyCmd(os.Args[2:])
    case "init":
        initCmd(os.Args[2:])
    case "tui":
        if err := ui.Run(); err != nil {
            fmt.Println(err)
        }
    case "-h", "--help", "help":
        usage()
    default:
        fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
        usage()
        os.Exit(2)
    }
}

func parseAgent(s string) (core.AgentID, error) {
    switch strings.ToLower(s) {
    case string(core.AgentClaude):
        return core.AgentClaude, nil
    case string(core.AgentGemini):
        return core.AgentGemini, nil
    case string(core.AgentCodex):
        return core.AgentCodex, nil
    default:
        return "", fmt.Errorf("invalid agent: %s", s)
    }
}

func listCmd(args []string) {
    fs := flag.NewFlagSet("list", flag.ExitOnError)
    agentFlag := fs.String("agent", "", "agent id: claude|gemini|codex")
    _ = fs.Parse(args)
    if *agentFlag == "" {
        fmt.Fprintln(os.Stderr, "--agent is required")
        os.Exit(2)
    }
    agent, err := parseAgent(*agentFlag)
    if err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(2)
    }

    prov := providers.NewProvider(agent)
    if prov == nil {
        fmt.Fprintln(os.Stderr, "provider not available for agent")
        os.Exit(1)
    }
    fields, err := prov.Read(context.Background())
    stat := "OK"
    if err != nil {
        fmt.Fprintf(os.Stderr, "read error: %v\n", err)
        stat = "Error"
    }
    fmt.Printf("Agent: %s\n", agent)
    fmt.Printf("Current: url=%s token=%s\n", fields.URL, util.Mask(fields.Token))
    if stat != "" {
        fmt.Printf("Status: %s\n", stat)
    }
    presets, _ := store.LoadPresets(agent)
    if len(presets) == 0 {
        fmt.Println("Presets: (none)")
        return
    }
    fmt.Println("Presets:")
    for _, p := range presets {
        fmt.Printf("  - %s: url=%s token=%s\n", p.Alias, p.URL, util.Mask(p.Token))
    }
}

func presetsCmd(args []string) {
    if len(args) < 1 {
        fmt.Fprintln(os.Stderr, "presets subcommand required: list|add")
        os.Exit(2)
    }
    sub := args[0]
    switch sub {
    case "list":
        fs := flag.NewFlagSet("presets list", flag.ExitOnError)
        agentFlag := fs.String("agent", "", "agent id")
        _ = fs.Parse(args[1:])
        if *agentFlag == "" {
            fmt.Fprintln(os.Stderr, "--agent is required")
            os.Exit(2)
        }
        agent, err := parseAgent(*agentFlag)
        if err != nil {
            fmt.Fprintln(os.Stderr, err)
            os.Exit(2)
        }
        presets, _ := store.LoadPresets(agent)
        for _, p := range presets {
            fmt.Printf("%s\t%s\t%s\n", p.Alias, p.URL, util.Mask(p.Token))
        }
    case "add":
        fs := flag.NewFlagSet("presets add", flag.ExitOnError)
        agentFlag := fs.String("agent", "", "agent id")
        alias := fs.String("alias", "", "preset alias (optional)")
        url := fs.String("url", "", "base url")
        token := fs.String("token", "", "api token (optional)")
        _ = fs.Parse(args[1:])
        if *agentFlag == "" || *url == "" {
            fmt.Fprintln(os.Stderr, "--agent and --url are required")
            os.Exit(2)
        }
        agent, err := parseAgent(*agentFlag)
        if err != nil {
            fmt.Fprintln(os.Stderr, err)
            os.Exit(2)
        }
        a := *alias
        if a == "" {
            a = time.Now().Format("20060102-1504")
        }
        if err := core.ValidateFields(core.Fields{URL: *url, Token: *token}); err != nil {
            fmt.Fprintln(os.Stderr, err)
            os.Exit(2)
        }
        if err := store.AddPreset(agent, core.Preset{Alias: a, URL: *url, Token: *token, AddedAt: time.Now().Format("20060102-1504")}); err != nil {
            fmt.Fprintln(os.Stderr, err)
            os.Exit(1)
        }
        fmt.Println("added")
    default:
        fmt.Fprintf(os.Stderr, "unknown presets subcommand: %s\n", sub)
        os.Exit(2)
    }
}

func applyCmd(args []string) {
    fs := flag.NewFlagSet("apply", flag.ExitOnError)
    agentFlag := fs.String("agent", "", "agent id")
    alias := fs.String("alias", "", "preset alias")
    url := fs.String("url", "", "base url (alternative to --alias)")
    token := fs.String("token", "", "api token (optional)")
    dry := fs.Bool("dry-run", false, "do not write, only show diff")
    _ = fs.Parse(args)
    if *agentFlag == "" {
        fmt.Fprintln(os.Stderr, "--agent is required")
        os.Exit(2)
    }
    agent, err := parseAgent(*agentFlag)
    if err != nil { fmt.Fprintln(os.Stderr, err); os.Exit(2) }

    var f core.Fields
    if *alias != "" {
        p, err := store.GetPreset(agent, *alias)
        if err != nil {
            fmt.Fprintln(os.Stderr, err)
            os.Exit(1)
        }
        f = core.Fields{URL: p.URL, Token: p.Token}
    } else if *url != "" {
        f = core.Fields{URL: *url, Token: *token}
    } else {
        fmt.Fprintln(os.Stderr, "either --alias or --url is required")
        os.Exit(2)
    }

    if err := core.ValidateFields(f); err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(2)
    }

    prov := providers.NewProvider(agent)
    if prov == nil { fmt.Fprintln(os.Stderr, "provider not available"); os.Exit(1) }
    old, _ := prov.Read(context.Background())
    diff := core.Diff(old, f)
    fmt.Println(diff)
    if *dry {
        return
    }
    if _, err := prov.Write(context.Background(), f); err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
    fmt.Println("applied")
}

func initCmd(args []string) {
    fs := flag.NewFlagSet("init", flag.ExitOnError)
    agentFlag := fs.String("agent", "", "agent id (optional; if omitted, run for all)")
    alias := fs.String("alias", "snap-default", "preset alias (default: snap-default)")
    _ = fs.Parse(args)
    var agents []core.AgentID
    if *agentFlag == "" {
        agents = []core.AgentID{core.AgentClaude, core.AgentGemini, core.AgentCodex}
    } else {
        a, err := parseAgent(*agentFlag)
        if err != nil { fmt.Fprintln(os.Stderr, err); os.Exit(2) }
        agents = []core.AgentID{a}
    }
    errCount := 0
    for _, agent := range agents {
        prov := providers.NewProvider(agent)
        if prov == nil {
            fmt.Fprintf(os.Stderr, "[%s] provider not available\n", agent)
            errCount++
            continue
        }
        cur, err := prov.Read(context.Background())
        if err != nil {
            fmt.Fprintf(os.Stderr, "[%s] read error: %v\n", agent, err)
            errCount++
            continue
        }
        if err := core.ValidateFields(cur); err != nil {
            fmt.Fprintf(os.Stderr, "[%s] skip: current config invalid (%v)\n", agent, err)
            continue
        }
        presets, _ := store.LoadPresets(agent)
        duplicate := false
        for _, p := range presets {
            if p.URL == cur.URL && p.Token == cur.Token {
                fmt.Printf("[%s] identical preset already exists (alias: %s), skipped\n", agent, p.Alias)
                duplicate = true
                break
            }
        }
        if duplicate { continue }
        a := *alias
        if _, err := store.GetPreset(agent, a); err == nil {
            a = a + "-" + time.Now().Format("20060102-1504")
            fmt.Fprintf(os.Stderr, "[%s] alias already exists, using %s instead\n", agent, a)
        }
        pr := core.Preset{Alias: a, URL: cur.URL, Token: cur.Token, AddedAt: time.Now().Format("20060102-1504")}
        if err := store.AddPreset(agent, pr); err != nil {
            fmt.Fprintf(os.Stderr, "[%s] add preset error: %v\n", agent, err)
            errCount++
            continue
        }
        fmt.Printf("[%s] added preset '%s'\n", agent, a)
    }
    if errCount > 0 && len(agents) > 1 {
        os.Exit(1)
    }
}
