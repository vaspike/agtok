# 1. 简介

agtok 是一个用于集中管理并一键切换三类常用 CLI 的 token 与 base URL 的工具：claude-code、gemini-cli、codex-cli。它同时支持交互式 TUI 与命令行（CLI），按 Agent 分组管理预设（Preset），支持预览与应用，写入过程原子且带备份。

# 2. 预览图

![TUI](image.png)

# 3. 快速开始

## 安装agtok

待补充

## 初始化预设,在你常用的终端中运行:

```bash 
agtok init
```
## 打开管理预设TUI:

```bash
agtok
```

# 4. 功能介绍

- 预设结构
  - 预设按 Agent 分文件存储于 `~/.config/token-switcher/presets/`
  - 示例（`claude.json`）：
  ```json
  { "version": 1, "presets": [
    { "alias": "dev", "url": "https://...", "token": "sk-...", "added_at": "20251031-0945" }
  ]}
  ```
  - TUI 中按 `p` 可在顶部 Status 显示预设目录路径

- 初始化预设
  - TUI：在某个 Agent 表格按 `i`，从当前磁盘配置生成预设（默认别名 `snap-default`，重名自动加时间戳）；自动去重
  - CLI：`agtok init [--agent <id>] [--alias <name>]`

- 添加预设
  - TUI：按 `a` 打开表单（URL 必填、Alias 可空、Token 可选），回车保存
  - CLI：`agtok presets add --agent <id> [--alias <name>] --url <u> [--token <t>]`

- 应用预设到 Agent 配置
  - TUI：选中某条预设，按 `Enter`；写入原子且带备份，权限 0600；Claude 仅写入 `ANTHROPIC_AUTH_TOKEN`
  - CLI：`agtok apply --agent <id> --alias <name> [--dry-run]` 或 `agtok apply --agent <id> --url <u> [--token <t>]`

- 重命名/删除预设
  - TUI：`e` 重命名（校验唯一与格式），`d` 删除（二次确认）；Active 行不可删除

- 版本检测
  - TUI：各 Agent 的 active 行第一列展示版本号；未安装显示 `Not installed`，无法解析显示 `Unknown`
  - 检测命令：`claude -v` / `gemini -v` / `codex -V`；异步回填、缓存 60s

- 运行方式
  - TUI：不带参数运行 `agtok` 即进入 TUI；或显式 `agtok tui`
  - CLI：传入子命令与参数时生效（list/apply/presets/init）

# 5. 支持的 Agent

- Claude-code（agent id: `claude`）
  - 路径：`~/.claude/settings.json`
  - 键：读取 `env.ANTHROPIC_AUTH_TOKEN`/`_API_TOKEN`/`_API_KEY`；写入仅 `_AUTH_TOKEN`

- Gemini-cli（agent id: `gemini`）
  - 路径：`~/.gemini/.env`
  - 键：`GOOGLE_GEMINI_BASE_URL`、`GEMINI_API_KEY`

- Codex-cli（agent id: `codex`）
  - 路径：`~/.codex/config.toml`（`model_providers.codex.base_url`）、`~/.codex/auth.json`（`OPENAI_API_KEY`）

# 6. 支持的平台

- macOS、Linux：已完整支持（TUI/CLI、预设落盘、版本检测、原子写入）
- Windows：计划支持
