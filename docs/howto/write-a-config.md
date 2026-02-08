# How to write a bootstrap config

## Prerequisites

- booster built and available (see [How to install and build booster](install-and-build.md))

## Steps

1. Create a file named `bootstrap.yaml`:

```yaml
version: "1"

tasks: []
```

2. Add tasks to the `tasks` array. Each task needs an `action` and `args`:

```yaml
version: "1"

tasks:
  - action: dir.create
    args:
      - ${ home }/.config/nvim

  - action: pkg.install
    args:
      - neovim
      - git

  - action: symlink.create
    args:
      - source: ~/dotfiles/nvim/init.lua
        target: ${ home }/.config/nvim/init.lua
```

See [Actions reference](../reference/actions.md) for all available actions and their argument formats.

3. Validate your config with a dry run:

```bash
booster run --config ./bootstrap.yaml --dry-run
```

This prints the task list without executing anything.

4. Run it for real:

```bash
booster run --config ./bootstrap.yaml
```

## Next steps

- Add conditional execution with `when` fields -- see [How to add conditional tasks](add-conditional-tasks.md)
- Define profiles for different machines -- see [How to use profiles](use-profiles.md)
- Collect user input with variables -- see [How to use variables and prompts](use-variables.md)
- Full config schema -- see [Config reference](../reference/config.md)
