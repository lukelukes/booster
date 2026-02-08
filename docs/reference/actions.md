# Actions reference

Each task in a config file specifies an `action` and its `args`. This page documents all available actions.

All string values in args support expression interpolation (`${ ... }`). See [expressions](expressions.md).

---

## `dir.create`

Creates directories recursively.

**Args:** array of strings (directory paths)

Supports `~` expansion.

**Example:**

```yaml
- action: dir.create
  args:
    - ~/dev
    - ~/.config
    - ~/.local/bin
```

---

## `symlink.create`

Creates symbolic links. Parent directories of the target are created automatically.

**Args:** array of objects

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `source` | string | yes | Path to the source file (must exist) |
| `target` | string | yes | Path of the symlink to create |

**Example:**

```yaml
- action: symlink.create
  args:
    - source: ~/dotfiles/.bashrc
      target: ~/.bashrc
    - source: ~/dotfiles/.gitconfig
      target: ~/.gitconfig
```

---

## `template.render`

Renders Go templates to files. Skips rendering when the output file already matches.

**Args:** array of objects

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `source` | string | yes | Path to the `.tmpl` template file |
| `target` | string | yes | Path for the rendered output |

Templates receive a `TemplateContext` with the following fields:

| Field | Access | Type |
|-------|--------|------|
| Variables | `.Vars` | `map[string]string` |
| OS | `.System.OS` | `string` |
| Profile | `.System.Profile` | `string` |

**Example:**

```yaml
- action: template.render
  args:
    - source: ~/dotfiles/gitconfig.tmpl
      target: ~/.gitconfig
```

---

## `pkg.install`

Installs system packages. Skips packages that are already installed.

Uses `pacman` (via an AUR helper) on Linux and `homebrew` on macOS.

**Args:** array of items, where each item is either:

- a string (package name), or
- an object with `packages` and/or `casks`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `packages` | array of strings | no | Package names |
| `casks` | array of strings | no | Homebrew casks (macOS only) |

**Example:**

```yaml
- action: pkg.install
  args:
    - git
    - ripgrep
    - packages:
        - neovim
        - tmux
      casks:
        - firefox
        - ghostty
```

---

## `pkg-manager.install`

Installs a package manager itself.

**Args:** array of strings

Allowed values: `paru`, `yay`, `homebrew`

**Example:**

```yaml
- action: pkg-manager.install
  args:
    - paru
```

---

## `mise.use`

Installs tools at specific versions via [mise](https://mise.jdx.dev/). Runs `mise use --global` for each tool.

Requires `mise` to be available in `PATH`.

**Args:** array of strings in `tool@version` format

Pattern: `^[a-zA-Z0-9_-]+@[a-zA-Z0-9._-]+$`

**Example:**

```yaml
- action: mise.use
  args:
    - go@1.22.0
    - node@20.10.0
    - rust@1.75.0
```

---

## `git.config`

Sets global git configuration values. Skips keys that already have the desired value.

**Args:** array of objects

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `key` | string | yes | Git config key |
| `value` | string | no | Value to set |
| `prompt` | string | no | Prompt text shown to the user at runtime |

Provide `value` for a static value. Provide `prompt` to ask the user interactively. When `prompt` is used and the key already has a value, the prompt is skipped.

**Example:**

```yaml
- action: git.config
  args:
    - key: user.name
      prompt: "Enter your full name"
    - key: user.email
      prompt: "Enter your email"
    - key: init.defaultBranch
      value: main
```

---

## `set.darwin.defaults`

Sets macOS system defaults via the `defaults` command. Skipped on non-macOS systems.

**Args:** object

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `file` | string | yes | Path to a YAML file containing defaults entries |

The YAML file contains a `defaults` array. Each entry has:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `domain` | string | yes | macOS defaults domain |
| `key` | string | yes | Preference key |
| `type` | string | yes | Value type: `bool`, `int`, `float`, `string` |
| `value` | any | yes | Value to set |

**Example:**

```yaml
- action: set.darwin.defaults
  args:
    file: macos-defaults.yaml
```

Defaults file (`macos-defaults.yaml`):

```yaml
defaults:
  - domain: NSGlobalDomain
    key: AppleShowAllExtensions
    type: bool
    value: true
  - domain: com.apple.dock
    key: tilesize
    type: int
    value: 48
```
