<!-- markdownlint-disable MD033 MD041 -->

```text
         ╔═══╗           █████ █   █ █████  ███  █   █ █████
    ╔════╩═══╩════╗        █   ██  █   █   █   █ █  █  █    
    ║  ─────────  ║        █   █ █ █   █   █████ ███   ████ 
    ╠═════════════╣        █   █  ██   █   █   █ █  █  █    
    ║ q w e r t y ║      █████ █   █   █   █   █ █   █ █████
    ║   a s d f   ║
    ╠═════════════╣      ━━━━━━━━━━━━━ ISSUE FORGE ━━━━━━━━━━━━━
    ║ [  space  ] ║      Ollama · GitHub CLI · Bubble Tea
    ╚═════════════╝      paste anything → structured issue,
    ═════════════════    one command. MIT · local AI · no cloud.
```

<p align="center">
  <a href="https://github.com/tinydarkforge/Intake/actions/workflows/ci.yml"><img alt="CI" src="https://github.com/tinydarkforge/Intake/actions/workflows/ci.yml/badge.svg"></a>
  <a href="https://github.com/tinydarkforge/Intake/releases/latest"><img alt="release" src="https://img.shields.io/github/v/release/tinydarkforge/Intake?style=flat-square&labelColor=0a0a0a&color=00cc66"></a>
  <a href="LICENSE"><img alt="license" src="https://img.shields.io/badge/license-MIT-00cc66.svg?style=flat-square&labelColor=0a0a0a"></a>
  <img alt="go" src="https://img.shields.io/badge/go-1.26%2B-00cc66.svg?style=flat-square&labelColor=0a0a0a">
  <a href="https://goreportcard.com/report/github.com/tinydarkforge/intake"><img alt="go report" src="https://goreportcard.com/badge/github.com/tinydarkforge/intake"></a>
  <a href="SECURITY.md"><img alt="security" src="https://img.shields.io/badge/security-policy-00cc66.svg?style=flat-square&labelColor=0a0a0a"></a>
</p>

## ░▒▓█ Overview

**Intake** is an agentic Terminal UI designed to bridge the gap between messy engineering context and structured GitHub issues. Powered by **local AI (Ollama)**, it allows you to transform Slack messages, error logs, and local file contents into professional issue drafts without ever leaving your terminal or sending data to the cloud.

> **Privacy First:** No cloud. No accounts. No telemetry. Your code and context never leave your machine.

---

## ░▒▓█ Key Features

- **Agentic Issue Drafting:** Paste raw context and let the agent infer titles, reproduction steps, and technical summaries using your repo's own templates.
- **Deep Context Integration:** Attach local files or logs directly from the filesystem pane to provide the AI with the exact code it needs to reference.
- **Interactive Refinement:** Don't like a draft? Chat with the agent to refine it (e.g., *"Make it more concise"* or *"Add a security section"*).
- **Dual-Pane Navigation:** A Norton Commander-style layout with a local filesystem browser on the left and GitHub issues on the right.
- **Workflow Bridge:** Instantly create local feature branches (e.g., `feat/123-fix-auth`) immediately after issue creation.
- **Live System Editing:** Jump into your preferred `$EDITOR` (vim/nano/etc.) to perform surgical tweaks to any AI-generated draft.
- **Rich Visualization:** Beautiful TUI rendering with Git status indicators and full Markdown support via Glamour.

---

## ░▒▓█ Installation

The guided installer handles dependencies (Ollama, GitHub CLI), pulls the default model, and configures your repository in one go.

```bash
curl -fsSL https://raw.githubusercontent.com/tinydarkforge/Intake/main/scripts/install.sh | bash
```

*Supports macOS and Linux. For Windows, please download the latest release from the [Releases Page](https://github.com/tinydarkforge/Intake/releases/latest).*

---

## ░▒▓█ The "Intake" Workflow

1. **Browse & Attach:** Navigate your local files. Press `a` to attach relevant code as context.
2. **Initiate:** Press `c` to create an issue. Choose a template.
3. **Contextualize:** Paste a Slack thread or error log.
4. **Refine:** Review the AI draft. Press `f` to provide follow-up instructions or `v` to edit manually in Vim.
5. **Ship:** Press `y` to create the GitHub issue.
6. **Bridge:** Press `b` to checkout a new local branch and start coding.

---

## ░▒▓█ Positioning

Intake is not a replacement for full-scale project management tools; it is a **high-speed triage gate**. It is built for the engineer who discovers a bug or a task and wants it documented and branched in under 30 seconds.

| Feature | Intake | GitHub Web | `gh issue create` |
|---------|--------|------------|-------------------|
| **Local Context** | Deep (Attached files) | None | Manual Copy/Paste |
| **AI Drafting** | Local (Private) | Optional (Cloud) | None |
| **Workflow** | TUI (Fast) | GUI (Slow) | CLI (Static) |
| **Data Privacy** | 100% Local | Cloud-processed | N/A |

---

## ░▒▓█ Requirements

- **[Ollama](https://ollama.ai):** Local AI engine (requires `llama3` or similar).
- **[GitHub CLI (`gh`)](https://cli.github.com):** Core API and authentication layer.
- **Go 1.26+:** (Only required for building from source).

---

## ░▒▓█ Configuration

### Settings Screen
Press `s` within the app to configure your repository, model choice, and Ollama host URL.

### Environment Variables
For power users, Intake respects the following:
- `INTAKE_REPO`: `owner/repo`
- `INTAKE_MODEL`: (Default: `llama3`)
- `OLLAMA_HOST`: (Default: `http://localhost:11434`)
- `EDITOR`: Your preferred editor for live-editing drafts.

---

## ░▒▓█ Usage Reference

### Global Keys
| Key | Action |
|-----|--------|
| `Tab` | Switch active pane |
| `c` | Create new issue |
| `s` | Open settings |
| `?` | Show help |
| `q` | Quit |

### Filesystem Pane (Left)
| Key | Action |
|-----|--------|
| `a` | **Attach/Detach file context** |
| `r` | Refresh (reloads Git status) |
| `enter` | Enter directory |

### Create Screen (Drafting)
| Key | Action |
|-----|--------|
| `ctrl+s` | Generate/Submit |
| `f` | **Refine with follow-up instruction** |
| `v` | **Edit in system editor** |
| `y` | Confirm and create on GitHub |
| `b` | **Create local feature branch** (after creation) |

---

## ░▒▓█ Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for details.

## ░▒▓█ License

Intake is released under the [MIT License](LICENSE).
