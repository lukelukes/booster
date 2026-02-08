# CLI reference

Bootstrap your machine from YAML config.

```
booster [--config <path>] <command> [options]
```

## Global flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--config` | path | `./bootstrap.yaml` | Path to config file |

## `run`

The default command. Invoked when no command is specified.

**Synopsis:**

```
booster run [--dry-run] [--profile <name>]
```

Loads the config file, builds the task list, and executes tasks in a TUI (bubbletea alt-screen mode). If any tasks require sudo, a password prompt appears before the TUI starts.

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--dry-run` | bool | `false` | Show what would be done without executing |
| `--profile` | string | | Profile to use (required when profiles are defined in config) |

**Profile validation:**

- If the config defines profiles and `--profile` is omitted, the command exits with an error listing available profiles.
- If `--profile` is specified but the config defines no profiles, the command exits with an error.
- If `--profile` is specified but does not match a configured profile, the command exits with an error.

**Dry-run output format:**

```
Would execute <N> task(s):

  1. <task-name>
  2. <task-name>
```

**Zero tasks:** When no tasks match after filtering, prints `No tasks to run` and exits with code 0.

## `version`

**Synopsis:**

```
booster version
```

Prints version, commit hash, and build date to stdout.

**Output format:**

```
cli <version> (commit: <commit>, built: <date>)
```

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Error (config load failure, profile validation, sudo failure, TUI error) |
