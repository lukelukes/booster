# How to use profiles

## Prerequisites

- A working `bootstrap.yaml` (see [How to write a bootstrap config](write-a-config.md))

## Steps

1. Add a `profiles` array at the root of your config:

```yaml
version: "1"

profiles: [personal, work]
```

2. Add `when` conditions to tasks that should only run for specific profiles:

```yaml
tasks:
  - action: dir.create
    args:
      - ${ home }/.config/nvim

  - action: git.config
    when: ${ profile == "work" }
    args:
      - key: user.email
        value: work@company.com

  - action: git.config
    when: ${ profile == "personal" }
    args:
      - key: user.email
        value: me@personal.com
```

Tasks without a `when` field run for all profiles.

3. Run with a profile:

```bash
booster run --config ./bootstrap.yaml --profile work
```

If you omit `--profile` when profiles are defined, booster exits with an error listing available profiles.

4. To match multiple profiles in one condition, use `in`:

```yaml
when: ${ profile in ["personal", "work"] }
```

See [Expressions reference](../reference/expressions.md) for the full expression syntax.
