# How to use variables and prompts

## Prerequisites

- A working `bootstrap.yaml` (see [How to write a bootstrap config](write-a-config.md))

## Steps

1. Add a `variables` section to your config. Each variable has a `prompt` and an optional `default`:

```yaml
version: "1"

variables:
  GitEmail:
    prompt: "Git email"
    default: "user@example.com"
  GitName:
    prompt: "Your full name"
```

2. Reference variables in task args using `${ vars.Name }`:

```yaml
tasks:
  - action: git.config
    args:
      - key: user.email
        value: ${ vars.GitEmail }
      - key: user.name
        value: ${ vars.GitName }
```

3. Run booster. It prompts you interactively for each variable:

```bash
booster run --config ./bootstrap.yaml
```

Variables with a `default` value pre-fill the prompt; press Enter to accept.

4. Variable values are persisted to `~/.local/share/cli/values.yaml`. On subsequent runs, booster uses the persisted values instead of prompting again.

If you need to change a persisted value, edit `~/.local/share/cli/values.yaml` directly or delete it to re-trigger all prompts.

5. You can also use variables in `when` conditions:

```yaml
when: ${ vars.GitEmail != "" }
```

See [Expressions reference](../reference/expressions.md) for all available context variables and functions.
