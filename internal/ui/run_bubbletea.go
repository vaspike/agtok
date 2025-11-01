package ui

import (
    "context"
    "fmt"
    "regexp"
    "sort"
    "strings"
    "time"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/bubbles/textinput"
    "github.com/charmbracelet/lipgloss"

    core "tks/internal/core"
    verinfo "tks/internal/version"
    "tks/internal/providers"
    "tks/internal/store"
    "tks/internal/util"
)

type rowKind int

const (
    rowCurrent rowKind = iota
    rowPreset
)

type row struct {
    kind  rowKind
    alias string
    url   string
    token string
    added string // create time (AddedAt) for presets; empty for active
}

type group struct {
    id    core.AgentID
    rows  []row
    index int
    ver   string
    inst  bool
}

type mode int

const (
    modeTable mode = iota
    modeNew
    modeConfirmDel
    modeRename
)

type model struct {
    groups []group
    active int // index into groups

    // new preset form
    m       mode
    aliasIn textinput.Model
    urlIn   textinput.Model
    tokIn   textinput.Model
    formErr string

    status string
    width  int
    height int

    // delete confirmation state
    delAlias string

    // version cache (session-level)
    verCache map[core.AgentID]verState

    // rename state
    renameOld string
    renameIn  textinput.Model
}

type verState struct {
    text      string
    installed bool
    at        time.Time
}

var (
    styleHeader     = lipgloss.NewStyle().Bold(true)
    styleGroupTitle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
    styleGreen      = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
    styleAliasSel   = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
    styleMuted      = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
    styleKey        = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
    styleStatusOK   = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
    styleStatusErr  = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
)

func initialModel() model {
    m := model{}
    m.m = modeTable
    m.aliasIn = textinput.New()
    m.urlIn = textinput.New()
    m.tokIn = textinput.New()
    m.urlIn.Placeholder = "https://..."
    m.aliasIn.Placeholder = "(optional)"
    m.tokIn.Placeholder = "(optional)"
    m.urlIn.Focus()
    m.verCache = map[core.AgentID]verState{}
    m.renameIn = textinput.New()
    m.renameIn.Placeholder = "new-alias"
    m.reloadAll()
    return m
}

const verTTL = 60 * time.Second

func (m *model) reloadAll() {
    m.groups = nil
    ids := []core.AgentID{core.AgentClaude, core.AgentGemini, core.AgentCodex}
    for _, id := range ids {
        g := group{id: id}
        var f core.Fields
        if prov := providers.NewProvider(id); prov != nil {
            f, _ = prov.Read(context.Background())
        }
        // load presets and detect active preset by value
        ps, _ := store.LoadPresets(id)
        sort.Slice(ps, func(i, j int) bool { return ps[i].Alias < ps[j].Alias })
        activeAlias := ""
        activeAdded := ""
        filtered := make([]storePreset, 0, len(ps))
        for _, p := range ps {
            if p.URL == f.URL && p.Token == f.Token {
                activeAlias = p.Alias
                activeAdded = p.AddedAt
                continue // do not duplicate in list
            }
            filtered = append(filtered, storePreset{Alias: p.Alias, URL: p.URL, Token: p.Token, AddedAt: p.AddedAt})
        }
        // version: prefer cached within TTL; otherwise show loading placeholder
        if st, ok := m.verCache[id]; ok && time.Since(st.at) < verTTL {
            g.ver = st.text
            g.inst = st.installed
        } else {
            g.ver = "…"
            g.inst = true
        }
        // active row at top
        g.rows = append(g.rows, row{kind: rowCurrent, alias: activeAlias, url: f.URL, token: f.Token, added: activeAdded})
        // preset rows
        for _, p := range filtered {
            g.rows = append(g.rows, row{kind: rowPreset, alias: p.Alias, url: p.URL, token: p.Token, added: p.AddedAt})
        }
        m.groups = append(m.groups, g)
    }
    m.active = 0
    m.status = "Loaded"
}

func (m model) Init() tea.Cmd { return m.scheduleVersionCmds() }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.width, m.height = msg.Width, msg.Height
        return m, nil
    case tea.KeyMsg:
        switch m.m {
        case modeTable:
            return m.updateTableKey(msg)
        case modeNew:
            return m.updateNewKey(msg)
        case modeConfirmDel:
            return m.updateConfirmKey(msg)
        case modeRename:
            return m.updateRenameKey(msg)
        }
    case verMsg:
        // async version backfill
        m.verCache[msg.id] = verState{text: msg.text, installed: msg.installed, at: msg.at}
        for i := range m.groups {
            if m.groups[i].id == msg.id {
                m.groups[i].ver = msg.text
                m.groups[i].inst = msg.installed
            }
        }
        return m, nil
    }
    return m, nil
}

func (m model) updateTableKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
    g := &m.groups[m.active]
    switch msg.String() {
    case "up", "k":
        if g.index > 0 { g.index-- }
    case "down", "j":
        if g.index < len(g.rows)-1 { g.index++ }
    case "1", "2", "3":
        m.active = int(msg.Runes[0]-'1')
        if m.active < 0 || m.active >= len(m.groups) { m.active = 0 }
        m.groups[m.active].index = 0
    case "r":
        m.reloadAll()
        return m, m.scheduleVersionCmds()
    case "enter":
        sel := g.rows[g.index]
        if sel.kind == rowPreset {
            // apply preset
            if prov := providers.NewProvider(g.id); prov != nil {
                old, _ := prov.Read(context.Background())
                diff := core.Diff(old, core.Fields{URL: sel.url, Token: sel.token})
                if _, err := prov.Write(context.Background(), core.Fields{URL: sel.url, Token: sel.token}); err != nil {
                    m.status = fmt.Sprintf("apply failed: %v", err)
                } else {
                    m.status = "applied"
                }
                _ = diff // reserved: show in detail in future
                // refresh current row
                m.reloadAll()
                return m, m.scheduleVersionCmds()
            }
        } else {
            m.status = "cannot apply active row"
        }
    case "a":
        m.m = modeNew
        m.formErr = ""
        m.aliasIn.SetValue("")
        m.urlIn.SetValue("")
        m.tokIn.SetValue("")
        m.urlIn.Focus(); m.aliasIn.Blur(); m.tokIn.Blur()
    case "p":
        // Show presets directory in status
        m.status = "Presets dir: " + store.PresetsDir()
    case "i":
        // Init from current disk config -> quick add preset for active agent
        prov := providers.NewProvider(g.id)
        if prov == nil {
            m.status = "init failed: provider not available"
            return m, nil
        }
        cur, err := prov.Read(context.Background())
        if err != nil {
            m.status = "init failed: " + err.Error()
            return m, nil
        }
        if err := core.ValidateFields(cur); err != nil {
            m.status = "skip: current config invalid"
            return m, nil
        }
        // duplicate by value
        ps, _ := store.LoadPresets(g.id)
        for _, p := range ps {
            if p.URL == cur.URL && p.Token == cur.Token {
                m.status = "identical preset exists: " + p.Alias
                return m, nil
            }
        }
        // alias selection: snap-default or timestamped if exists
        alias := "snap-default"
        if _, err := store.GetPreset(g.id, alias); err == nil {
            alias = alias + "-" + time.Now().Format("20060102-1504")
        }
        pr := core.Preset{Alias: alias, URL: cur.URL, Token: cur.Token, AddedAt: time.Now().Format("20060102-1504")}
        if err := store.AddPreset(g.id, pr); err != nil {
            m.status = "init failed: " + err.Error()
            return m, nil
        }
        m.status = "added preset '" + alias + "'"
        m.reloadAll()
        return m, m.scheduleVersionCmds()
    case "d":
        sel := g.rows[g.index]
        if sel.kind == rowCurrent {
            m.status = "cannot delete active row"
            return m, nil
        }
        m.m = modeConfirmDel
        m.delAlias = sel.alias
    case "e":
        sel := g.rows[g.index]
        // allow rename on preset rows or on active row that maps to a preset (has alias)
        if sel.kind == rowCurrent && sel.alias == "" {
            m.status = "no preset for active row"
            return m, nil
        }
        if sel.alias == "" {
            m.status = "no alias to rename"
            return m, nil
        }
        m.m = modeRename
        m.renameOld = sel.alias
        m.renameIn.SetValue(sel.alias)
        m.renameIn.CursorEnd()
        m.renameIn.Focus()
    case "q", "esc", "ctrl+c":
        return m, tea.Quit
    }
    return m, nil
}

func (m model) updateNewKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
    switch msg.String() {
    case "tab":
        if m.urlIn.Focused() {
            m.aliasIn.Focus()
            m.urlIn.Blur()
        } else if m.aliasIn.Focused() {
            m.tokIn.Focus()
            m.aliasIn.Blur()
        } else {
            m.urlIn.Focus()
            m.tokIn.Blur()
        }
    case "enter":
        // validate and add
        alias := strings.TrimSpace(m.aliasIn.Value())
        url := strings.TrimSpace(m.urlIn.Value())
        tok := m.tokIn.Value()
        if err := core.ValidateFields(core.Fields{URL: url, Token: tok}); err != nil {
            m.formErr = err.Error(); return m, nil
        }
        g := &m.groups[m.active]
        // duplicate by value
        ps, _ := store.LoadPresets(g.id)
        for _, p := range ps { if p.URL==url && p.Token==tok { m.formErr = "preset with same values exists: "+p.Alias; return m, nil } }
        if alias == "" { alias = time.Now().Format("20060102-1504") }
        // duplicate alias
        if _, err := store.GetPreset(g.id, alias); err == nil { m.formErr = "alias exists"; return m, nil }
        if err := store.AddPreset(g.id, core.Preset{Alias: alias, URL: url, Token: tok, AddedAt: time.Now().Format("20060102-1504")}); err != nil {
            m.formErr = err.Error(); return m, nil
        }
        m.m = modeTable
        m.status = "added"
        m.reloadAll()
        // focus new row
        gg := &m.groups[m.active]
        for i, r := range gg.rows { if r.kind==rowPreset && r.alias==alias { gg.index=i; break } }
    case "esc", "q":
        m.m = modeTable
    default:
        // delegate inputs
        var cmd tea.Cmd
        if m.urlIn.Focused() { m.urlIn, cmd = m.urlIn.Update(msg); return m, cmd }
        if m.aliasIn.Focused() { m.aliasIn, cmd = m.aliasIn.Update(msg); return m, cmd }
        if m.tokIn.Focused() { m.tokIn, cmd = m.tokIn.Update(msg); return m, cmd }
    }
    return m, nil
}

func (m model) updateConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
    switch msg.String() {
    case "y", "Y":
        g := m.groups[m.active]
        if err := store.RemovePreset(g.id, m.delAlias); err != nil {
            m.status = "delete failed: " + err.Error()
        } else {
            m.status = "deleted"
            m.reloadAll()
            return m, m.scheduleVersionCmds()
        }
        m.m = modeTable
        m.delAlias = ""
    case "n", "esc", "q":
        m.m = modeTable
        m.delAlias = ""
    }
    return m, nil
}

var aliasRe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,32}$`)

func (m model) updateRenameKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
    switch msg.String() {
    case "enter":
        g := m.groups[m.active]
        old := m.renameOld
        newA := strings.TrimSpace(m.renameIn.Value())
        if newA == old {
            m.status = "alias unchanged"
            m.m = modeTable
            return m, nil
        }
        if !aliasRe.MatchString(newA) {
            m.formErr = "invalid alias (allowed: A-Za-z0-9_- , len 1-32)"
            return m, nil
        }
        if err := store.RenamePreset(g.id, old, newA); err != nil {
            m.formErr = err.Error()
            return m, nil
        }
        m.status = fmt.Sprintf("renamed '%s' -> '%s'", old, newA)
        m.m = modeTable
        m.reloadAll()
        // focus new alias
        gg := &m.groups[m.active]
        for i, r := range gg.rows { if r.kind==rowPreset && r.alias==newA { gg.index=i; break } }
        return m, nil
    case "esc", "q":
        m.m = modeTable
        m.formErr = ""
        return m, nil
    default:
        var cmd tea.Cmd
        m.renameIn, cmd = m.renameIn.Update(msg)
        return m, cmd
    }
}

func (m model) View() string {
    // top bar: app name + version + status (single-line if fits, otherwise 2 lines)
    top := m.renderTop()
    // tables full width
    tables := m.renderTable()
    // bottom details
    details := m.renderDetailBottom()
    return top + "\n" + tables + details + "\n" + m.help()
}

func (m model) renderTop() string {
    name := verinfo.Name
    if name == "" { name = "agtok" }
    ver := verinfo.Version
    if ver == "" { ver = "dev" }
    leftRaw := name + " " + ver
    left := styleHeader.Render(name) + " " + styleMuted.Render(ver)
    // color status: success vs error
    st := m.status
    stStyled := styleStatusOK.Render(st)
    ls := strings.ToLower(st)
    if strings.Contains(ls, "failed") || strings.Contains(ls, "error") || strings.Contains(ls, "cannot") {
        stStyled = styleStatusErr.Render(st)
    }
    rightRaw := "Status: " + st
    sep := " | "
    // decide single vs double line based on raw rune widths
    rawLen := runeLen(leftRaw + sep + rightRaw)
    if m.width > 0 && rawLen > m.width {
        // two lines; second line status may be truncated
        // keep some padding for label
        avail := m.width - len([]rune("Status: "))
        if avail < 8 { avail = 8 }
        return left + "\n" + "Status: " + styleStatusOK.Render(truncate(st, avail))
    }
    return left + sep + "Status: " + stStyled
}

func runeLen(s string) int { return len([]rune(s)) }

func (m model) help() string {
    // Build a more readable footer help; switch content by mode
    var b strings.Builder
    if m.m == modeNew {
        // Add mode: show target agent and form-related keys
        g := m.groups[m.active]
        b.WriteString("Add: ")
        b.WriteString(styleKey.Render(agentTitle(g.id)))
        b.WriteString("  ")
        b.WriteString(styleKey.Render("[Tab]"))
        b.WriteString(" Next  ")
        b.WriteString(styleKey.Render("[Enter]"))
        b.WriteString(" Save  ")
        b.WriteString(styleKey.Render("[Esc]"))
        b.WriteString(" Cancel")
        return b.String()
    } else if m.m == modeConfirmDel {
        g := m.groups[m.active]
        b.WriteString("Delete: ")
        b.WriteString(styleKey.Render(agentTitle(g.id)))
        b.WriteString("  ")
        b.WriteString(styleKey.Render(m.delAlias))
        b.WriteString("  ")
        b.WriteString(styleKey.Render("[y]"))
        b.WriteString(" Yes  ")
        b.WriteString(styleKey.Render("[n/Esc]"))
        b.WriteString(" Cancel")
        return b.String()
    } else if m.m == modeRename {
        g := m.groups[m.active]
        b.WriteString("Rename: ")
        b.WriteString(styleKey.Render(agentTitle(g.id)))
        b.WriteString("  ")
        b.WriteString(styleKey.Render(m.renameOld))
        b.WriteString("  ")
        b.WriteString(styleKey.Render("[Enter]"))
        b.WriteString(" Save  ")
        b.WriteString(styleKey.Render("[Esc]"))
        b.WriteString(" Cancel")
        return b.String()
    }
    // Table mode
    // Agent group selector row with explicit mapping
    b.WriteString("Agent: ")
    b.WriteString(styleKey.Render("[1]"))
    b.WriteString(" ")
    b.WriteString(agentTitle(core.AgentClaude))
    b.WriteString("  ")
    b.WriteString(styleKey.Render("[2]"))
    b.WriteString(" ")
    b.WriteString(agentTitle(core.AgentGemini))
    b.WriteString("  ")
    b.WriteString(styleKey.Render("[3]"))
    b.WriteString(" ")
    b.WriteString(agentTitle(core.AgentCodex))
    b.WriteString("\n")
    // Actions row
    b.WriteString("Actions: ")
    b.WriteString(styleKey.Render("[↑/↓]"))
    b.WriteString(" Move  ")
    b.WriteString(styleKey.Render("[Enter]"))
    b.WriteString(" Apply  ")
    b.WriteString(styleKey.Render("[a]"))
    b.WriteString(" Add  ")
    b.WriteString(styleKey.Render("[i]"))
    b.WriteString(" Init  ")
    b.WriteString(styleKey.Render("[p]"))
    b.WriteString(" Path  ")
    b.WriteString(styleKey.Render("[e]"))
    b.WriteString(" Rename  ")
    b.WriteString(styleKey.Render("[d]"))
    b.WriteString(" Delete  ")
    b.WriteString(styleKey.Render("[r]"))
    b.WriteString(" Reload  ")
    b.WriteString(styleKey.Render("[q]"))
    b.WriteString(" Quit")
    return b.String()
}

func agentTitle(id core.AgentID) string {
    switch id {
    case core.AgentClaude:
        return "claude-code"
    case core.AgentGemini:
        return "gemini-cli"
    case core.AgentCodex:
        return "codex-cli"
    default:
        return string(id)
    }
}

// storePreset is a local mirror used to sort and filter
type storePreset struct{ Alias, URL, Token, AddedAt string }

// computeWidths for columns: Agent(header only), Active, Alias, URL.
func (m model) computeWidths() (wAgent, wActive, wAlias, wURL int) {
    // Fixed columns (agent names like "[1] claude-code" need a bit more room)
    wAgent, wActive, wAlias = 16, 6, 16
    minURL := 20
    // Overhead for 4 columns row: left bar+space (2) + 3 mids (" │ ", 9) + trailing space+right bar (2) = 13
    overhead := 13
    fixed := wAgent + wActive + wAlias
    // Target total table width is 70% of terminal width
    target := int(float64(m.width) * 0.7)
    // Ensure target is at least enough to hold minimal URL width
    minTarget := fixed + overhead + minURL
    if target < minTarget {
        target = minTarget
    }
    wURL = target - fixed - overhead
    if wURL < minURL {
        wURL = minURL
    }
    return
}

func (m model) renderTable() string {
    // widths for columns: Agent, Active, Alias, URL (adaptive URL)
    wAgent, wActive, wAlias, wURL := m.computeWidths()
    widths := []int{wAgent, wActive, wAlias, wURL}
    // helpers for box-drawing borders
    seg := func(w int) string { return strings.Repeat("─", w+2) }
    drawBorder := func(left, mid, right string, green bool) string {
        var b strings.Builder
        b.WriteString(left)
        for i, w := range widths {
            b.WriteString(seg(w))
            if i < len(widths)-1 { b.WriteString(mid) } else { b.WriteString(right) }
        }
        s := b.String()
        if green { return styleGreen.Render(s) }
        return s
    }
    // Use ASCII mark for Active
    check := func() string { return "*" }

    // build out
    var out strings.Builder
    for gi, g := range m.groups {
        // top border
        out.WriteString(drawBorder("╭", "┬", "╮", false) + "\n")
        // header row: first column shows agent name (blue if this table is active), others are column titles
        agentHdr := fmt.Sprintf("[%d] %s", gi+1, agentTitle(g.id))
        agentCell := fmt.Sprintf("%-*s", wAgent, agentHdr)
        if gi == m.active { agentCell = styleAliasSel.Render(agentCell) }
        activeCell := fmt.Sprintf("%-*s", wActive, "Active")
        aliasCell := fmt.Sprintf("%-*s", wAlias, "Alias")
        urlCell := fmt.Sprintf("%-*s", wURL, "URL")
        header := fmt.Sprintf("│ %s │ %s │ %s │ %s │\n", agentCell, activeCell, aliasCell, urlCell)
        out.WriteString(header)
        // header separator (underline), always visible (no color) to ensure consistent rendering
        out.WriteString(drawBorder("├", "┼", "┤", false) + "\n")

        // rows
        last := len(g.rows) - 1
        for i, r := range g.rows {
            isSel := gi == m.active && i == g.index
            activeMark := ""
            if r.kind == rowCurrent { activeMark = check() }
            // raw contents (truncated)
            aliasRaw := truncate(r.alias, wAlias)
            urlRaw := truncate(r.url, wURL)
            // pad each cell to fixed width first
            // Agent column: show version on active row only
            verText := ""
            if i == 0 { verText = g.ver }
            agentCell := fmt.Sprintf("%-*s", wAgent, truncate(verText, wAgent))
            if i == 0 && !g.inst {
                agentCell = styleMuted.Render(agentCell)
            }
            aCell := fmt.Sprintf("%-*s", wActive, truncate(activeMark, wActive))
            aliasCell := fmt.Sprintf("%-*s", wAlias, aliasRaw)
            urlCell := fmt.Sprintf("%-*s", wURL, urlRaw)
            if isSel {
                agentCell = styleAliasSel.Render(agentCell)
                aCell = styleAliasSel.Render(aCell)
                aliasCell = styleAliasSel.Render(aliasCell)
                urlCell = styleAliasSel.Render(urlCell)
            }
            // vertical bars: all default color; no colored borders for selection
            vbL := "│"
            vbR := "│"
            rowStr := fmt.Sprintf("%s %s │ %s │ %s │ %s %s\n",
                vbL, agentCell, aCell, aliasCell, urlCell, vbR)
            out.WriteString(rowStr)
            // row separator or bottom border
            if i < last {
                out.WriteString(drawBorder("├", "┼", "┤", false) + "\n")
            } else {
                out.WriteString(drawBorder("╰", "┴", "╯", false) + "\n")
            }
        }
        if gi < len(m.groups)-1 { out.WriteString("\n") }
    }
    return out.String()
}

func (m model) renderDetailBottom() string {
    g := m.groups[m.active]
    r := g.rows[g.index]
    var b strings.Builder
    b.WriteString(lipgloss.NewStyle().Bold(true).Render("\nDetails")+"\n")
    b.WriteString(fmt.Sprintf("Agent: %s\n", agentTitle(g.id)))
    // Active mark + alias + create time
    activeMark := ""
    if r.kind == rowCurrent { activeMark = "✔" }
    b.WriteString(fmt.Sprintf("Active: %s  Alias: %s  CreateTime: %s\n", activeMark, r.alias, r.added))
    b.WriteString(fmt.Sprintf("URL: %s\n", r.url))
    b.WriteString(fmt.Sprintf("Token: %s\n", util.Mask(r.token)))
    if m.m == modeNew {
        b.WriteString("\nAdd Preset for ")
        b.WriteString(agentTitle(g.id))
        b.WriteString(":\n")
        b.WriteString("URL:   "+m.urlIn.View()+"\n")
        b.WriteString("Alias: "+m.aliasIn.View()+"\n")
        b.WriteString("Token: "+m.tokIn.View()+"\n")
        if m.formErr != "" {
            b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(m.formErr)+"\n")
        }
    } else if m.m == modeConfirmDel {
        b.WriteString("\nConfirm Delete:\n")
        b.WriteString(fmt.Sprintf("Agent: %s\n", agentTitle(g.id)))
        b.WriteString(fmt.Sprintf("Preset: %s\n", m.delAlias))
        b.WriteString("Press 'y' to confirm, 'n' or 'Esc' to cancel.\n")
    } else if m.m == modeRename {
        b.WriteString("\nRename Preset:\n")
        b.WriteString(fmt.Sprintf("Agent: %s\n", agentTitle(g.id)))
        b.WriteString(fmt.Sprintf("Old: %s\n", m.renameOld))
        b.WriteString("New: "+m.renameIn.View()+"\n")
        if m.formErr != "" {
            b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(m.formErr)+"\n")
        }
    }
    return b.String()
}

// async version loading
type verMsg struct {
    id        core.AgentID
    text      string
    installed bool
    at        time.Time
}

func versionCmd(id core.AgentID) tea.Cmd {
    return func() tea.Msg {
        text, inst := detectVersion(id)
        return verMsg{id: id, text: text, installed: inst, at: time.Now()}
    }
}

func (m model) scheduleVersionCmds() tea.Cmd {
    var cmds []tea.Cmd
    ids := []core.AgentID{core.AgentClaude, core.AgentGemini, core.AgentCodex}
    now := time.Now()
    for _, id := range ids {
        st, ok := m.verCache[id]
        if !ok || now.Sub(st.at) >= verTTL {
            cmds = append(cmds, versionCmd(id))
        }
    }
    if len(cmds) == 0 { return nil }
    return tea.Batch(cmds...)
}

func truncate(s string, n int) string {
    if len(s) <= n { return s }
    if n <= 1 { return s[:n] }
    return s[:n-1] + "…"
}

// Run starts the TUI program (built with -tags tui)
func Run() error {
    p := tea.NewProgram(initialModel(), tea.WithAltScreen())
    _, err := p.Run()
    return err
}
