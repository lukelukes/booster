# Bootstrap your first machine

In this tutorial we will build a working bootstrap configuration from scratch, one piece at a time. By the end you will have a config that creates directories, installs packages conditionally, sets git config, and uses profiles to vary behavior.

## Prerequisites

Build the booster binary:

```
make build
```

The binary is at `out/booster`.

## Create a minimal config

Create a file called `bootstrap.yaml` in your working directory:

```yaml
version: "1"

tasks:
  - action: dir.create
    args:
      - ~/.projects
```

This declares one task: create the `~/.projects` directory.

## Run a dry run

```
out/booster run --config ./bootstrap.yaml --dry-run
```

The output should look like:

```
Would execute 1 task(s):

  1. create ~/.projects
```

The dry run shows what booster would do without making any changes.

## Add a conditional package install

We will now add a task that only runs on specific operating systems. Replace the contents of `bootstrap.yaml` with:

```yaml
version: "1"

tasks:
  - action: dir.create
    args:
      - ~/.projects

  - action: pkg.install
    when: ${ os == "arch" }
    args:
      - neovim
      - git
```

The `when` field is an [expression](../reference/expressions.md) that evaluates at runtime. This task will only run on Arch Linux.

Run the dry run again:

```
out/booster run --config ./bootstrap.yaml --dry-run
```

On an Arch Linux machine, the output should look like:

```
Would execute 2 task(s):

  1. create ~/.projects
  2. pkg.install
```

On any other OS, the `pkg.install` task is still listed but will be skipped at execution time. You will see the same 2 tasks in the dry-run output regardless of OS, because conditional tasks are evaluated lazily during execution.

## Add a variable and git config

Next we will add a user-defined variable and a `git.config` task that uses it. Replace the contents of `bootstrap.yaml` with:

```yaml
version: "1"

variables:
  GitEmail:
    prompt: "Git email"
    default: "user@example.com"

tasks:
  - action: dir.create
    args:
      - ~/.projects

  - action: pkg.install
    when: ${ os == "arch" }
    args:
      - neovim
      - git

  - action: git.config
    args:
      - key: user.email
        value: ${ vars.GitEmail }
```

The `variables` section defines a `GitEmail` variable. At runtime, booster prompts for its value and persists the answer. The `git.config` task references it via `${ vars.GitEmail }`.

Run the dry run:

```
out/booster run --config ./bootstrap.yaml --dry-run
```

You will see an interactive prompt asking for the git email value. After entering a value (or accepting the default), the output should look like:

```
Would execute 3 task(s):

  1. create ~/.projects
  2. pkg.install
  3. configure git: user.email
```

## Add profiles

We will now add profiles so that certain tasks only run for a specific profile. Replace the contents of `bootstrap.yaml` with:

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
      - ~/.projects

  - action: pkg.install
    when: ${ os == "arch" }
    args:
      - neovim
      - git

  - action: git.config
    args:
      - key: user.email
        value: ${ vars.GitEmail }

  - action: dir.create
    when: ${ profile == "work" }
    args:
      - ~/work
```

The `profiles` field declares which profiles are valid. The new `dir.create` task for `~/work` only runs when the `work` profile is selected.

When profiles are defined, the `--profile` flag is required. Run without it first:

```
out/booster run --config ./bootstrap.yaml --dry-run
```

You will see an error:

```
error: config defines profiles [personal work], use --profile to select one
```

Now run with a profile:

```
out/booster run --config ./bootstrap.yaml --dry-run --profile work
```

After the variable prompt, the output should look like:

```
Would execute 4 task(s):

  1. create ~/.projects
  2. pkg.install
  3. configure git: user.email
  4. dir.create
```

Run again with the `personal` profile:

```
out/booster run --config ./bootstrap.yaml --dry-run --profile personal
```

You will see the same 4 tasks listed. The `dir.create` task for `~/work` is still present but its `when` condition will evaluate to false at execution time, causing it to be skipped.

## Next steps

You have built a complete bootstrap config that creates directories, installs packages conditionally, configures git with user-provided variables, and uses profiles to vary behavior.

To go further:

- [Config reference](../reference/config.md) -- full schema documentation
- [Actions reference](../reference/actions.md) -- all available actions and their args
- [Expressions reference](../reference/expressions.md) -- expression syntax, context variables, and functions
- [Design and rationale](../explanation/design.md) -- why booster works the way it does
