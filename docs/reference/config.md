# Configuration Reference

The configuration file is `bootstrap.yaml`, located in the repository root. The full JSON Schema is available at `schema.json` in the repository root.

## Root Fields

| Field | Type | Required | Description |
|---|---|---|---|
| `version` | string | Yes | Schema version. Must be `"1"`. |
| `tasks` | array | Yes | List of [task objects](#task). |
| `profiles` | array of strings | No | Available profile names for conditional task execution. |
| `variables` | object | No | [Variable definitions](#variable) keyed by name. |

## Task

Each entry in `tasks` is an object with the following fields:

| Field | Type | Required | Description |
|---|---|---|---|
| `action` | string | Yes | Action to execute. One of: `dir.create`, `symlink.create`, `template.render`, `pkg.install`, `pkg-manager.install`, `mise.use`, `git.config`, `set.darwin.defaults`. See [Actions Reference](actions.md). |
| `args` | varies | Depends on action | Action-specific arguments. See [Actions Reference](actions.md). |
| `when` | string | No | Expression string in `${ ... }` form. The task runs only when the expression evaluates to true. See [Expressions Reference](expressions.md). |

## Variable

Each key under `variables` is the variable name. The value is an object with the following fields:

| Field | Type | Required | Description |
|---|---|---|---|
| `prompt` | string | Yes | Prompt text shown to the user to collect the value. |
| `default` | string | No | Default value pre-filled in the prompt. |

Variable values are collected via interactive prompts at runtime. Collected values are persisted to `$XDG_DATA_HOME/cli/values.yaml` (defaults to `~/.local/share/cli/values.yaml`).

## Example

```yaml
version: "1"

profiles: [personal, work]

variables:
  GitEmail:
    prompt: "Git email"
    default: "user@example.com"

tasks:
  - action: dir.create
    args:
      - ${ home }/.config/nvim

  - action: pkg.install
    when: ${ os in ["arch", "darwin"] }
    args:
      - neovim
      - git

  - action: git.config
    when: ${ profile == "work" }
    args:
      - key: user.email
        value: ${ vars.GitEmail }
```
