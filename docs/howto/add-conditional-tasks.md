# How to add conditional tasks

## Prerequisites

- A working `bootstrap.yaml` (see [How to write a bootstrap config](write-a-config.md))

## Steps

Add a `when` field to any task. The expression must evaluate to a boolean. Tasks without `when` always run.

### OS-based conditions

Run a task only on specific operating systems:

```yaml
- action: pkg.install
  when: ${ os == "arch" }
  args:
    - neovim

- action: pkg.install
  when: ${ os == "darwin" }
  args:
    - neovim
```

On macOS, `os` is `"darwin"`. On Linux, `os` is the distro ID from `/etc/os-release` (e.g., `"arch"`, `"ubuntu"`, `"fedora"`).

To match multiple OS values:

```yaml
when: ${ os in ["arch", "ubuntu", "fedora"] }
```

### Profile-based conditions

Run a task only for a specific profile:

```yaml
- action: git.config
  when: ${ profile == "work" }
  args:
    - key: user.email
      value: work@company.com
```

See [How to use profiles](use-profiles.md) for full setup.

### Checking if a tool is installed

```yaml
- action: pkg.install
  when: ${ not installed("git") }
  args:
    - git
```

### Checking if a file or directory exists

```yaml
- action: dir.create
  when: ${ not exists(home + "/.config/nvim") }
  args:
    - ${ home }/.config/nvim
```

### Combining conditions

Use `and`, `or`, and `not` to compose conditions:

```yaml
when: ${ os == "arch" and not installed("neovim") }
```

```yaml
when: ${ profile == "work" or profile == "personal" }
```

```yaml
when: ${ os == "darwin" and exists(home + "/.Brewfile") }
```

See [Expressions reference](../reference/expressions.md) for the full list of operators, context variables, and functions.
