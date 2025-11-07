![Windows](https://img.shields.io/badge/Windows-0078D6?style=for-the-badge&logo=windows&logoColor=white&color=0078D6)
![Linux](https://img.shields.io/badge/Linux-FCC624?style=for-the-badge&logo=linux&logoColor=white&color=FCC624)
![macOS](https://img.shields.io/badge/macOS-000000?style=for-the-badge&logo=apple&logoColor=white&color=000000)
![DevSwitcher2 Logo](https://img.shields.io/badge/agtok-1.0.1-blue?style=for-the-badge)
![GitHub Downloads (all assets, all releases)](https://img.shields.io/github/downloads/vaspike/agtok/total?style=for-the-badge&color=orange)

# 1. Introduction

agtok is a tool for centrally managing and switching tokens and base URLs for three common CLIs with a single command: claude-code, gemini-cli, and codex-cli. It supports both interactive TUI and command-line interface (CLI). It manages presets grouped by Agent, supports preview and application, and ensures atomic writes with backups.

# 2. Preview

![TUI](tui-preview.png)

# 3. Quick Start

## Install agtok

### Method 1: Brew install

```bash
# Install
brew tap vaspike/homebrew-agtok && brew install agtok

# Update
brew upgrade agtok
```
### Method 2: Download Release
1. Visit the [Releases page](https://github.com/vaspike/agtok/releases)
2. Download the corresponding release
3. Unpacking release
4. (macOS, Linux)Execute in a terminal(create a soft link):
   ```bash
   # create a soft link
   sudo ln -sfn "$PWD/agtok" /usr/local/bin/agtok

   # Or direct cp to `/usr/local/bin/`
   sudo cp -f "$PWD/agtok" /usr/local/bin
   ```
5. (Windows) Copy the `agtok.exe` file to the `C:\Program Files\agtok` directory

## Initialize Presets, run in your usual terminal:

```bash
agtok init
```
## Open the Preset Management TUI:

```bash
agtok
```

# 4. Features

- Preset Structure
  - Presets are stored by Agent in separate files under `~/.config/token-switcher/presets/`
  - Example (`claude.json`):
  ```json
  { "version": 1, "presets": [
    { "alias": "dev", "url": "https://...", "token": "sk-...", "model": "sonnet", "added_at": "20251031-0945" }
  ]}
  ```
  - In the TUI, press `p` to display the preset directory path in the top Status bar.

- Initialize Presets
  - TUI: In an Agent table, press `i` to generate a preset from the current disk configuration (default alias `snap-default`, automatically adds a timestamp if name conflicts); automatic deduplication.
  - CLI: `agtok init [--agent <id>] [--alias <name>]`

- Add Presets
  - TUI: Press `a` to open the form (URL is required, Alias can be empty, Token is optional), press Enter to save.
  - CLI: `agtok presets add --agent <id> [--alias <name>] --url <u> [--token <t>]`

- Update Presets (TUI)
  - TUI: Select a row and press `u` to update fields. URL left blank = unchanged; Token `-` = clear (preset only); blank = unchanged; for Claude, Model empty = clear, non-empty = set.
  - If updating the active row, Claude's Model on disk is strictly mirrored: empty removes `ANTHROPIC_MODEL`, non-empty writes/overwrites. Other agents update presets only.

- Apply Presets to Agent Configuration
  - TUI: Select a preset and press `Enter`; writes are atomic with backups, permissions 0600; Claude only writes `ANTHROPIC_AUTH_TOKEN`.
  - Claude Model: applying a Claude preset mirrors `ANTHROPIC_MODEL` on disk; if the preset has no model value, the key is removed; if it has a value, the key is written/overwritten.
  - CLI: `agtok apply --agent <id> --alias <name> [--dry-run]` or `agtok apply --agent <id> --url <u> [--token <t>]`

- Rename/Delete Presets
  - TUI: `e` to rename (validates uniqueness and format), `d` to delete (requires secondary confirmation); the active row cannot be deleted.

- Version Detection
  - TUI: The first column of each Agent's active row displays the version number; `Not installed` is shown if not installed, `Unknown` if parsing fails.
  - Detection commands: `claude -v` / `gemini -v` / `codex -V`; asynchronous backfill, cached for 60s. Gemini detection allows a slightly longer timeout.

- Status Bar & Details
  - Top bar shows `agtok <version>` and a colored Status (green for OK, red for errors). Press `p` to show the presets dir path in Status.
  - Details panel shows the selected row; for Claude, `Model` is displayed and `(not set)` appears in muted color when empty.

- Running Modes
  - TUI: Run `agtok` without parameters to enter TUI; or explicitly `agtok tui`.
  - CLI: Effective when subcommand and parameters are passed (list/apply/presets/init).

# 5. Supported Agents

- Claude-code (agent id: `claude`)
  - Path: `~/.claude/settings.json`
  - Keys: Reads `env.ANTHROPIC_AUTH_TOKEN`/`_API_TOKEN`/`_API_KEY`; only writes `_AUTH_TOKEN`.

- Gemini-cli (agent id: `gemini`)
  - Path: `~/.gemini/.env`
  - Keys: `GOOGLE_GEMINI_BASE_URL`, `GEMINI_API_KEY`.

- Codex-cli (agent id: `codex`)
  - Path: `~/.codex/config.toml` (`model_providers.codex.base_url`), `~/.codex/auth.json` (`OPENAI_API_KEY`).

# 6. Supported Platforms

- macOS, Linux: Fully supported (TUI/CLI, preset persistence, version detection, atomic writes).
- Windows: Planned support.
