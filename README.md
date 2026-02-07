# booster

Declarative machine bootstrap tool driven by YAML tasks.

## Build

```bash
make build
```

Binary path: `out/booster`

## Run

```bash
./out/booster run --config ./bootstrap.yaml
```

Dry run:

```bash
./out/booster run --config ./bootstrap.yaml --dry-run
```

If `profiles` are defined in config, pass one:

```bash
./out/booster run --config ./bootstrap.yaml --profile work
```

## Config Contract

Required root fields:

- `version: "1"`
- `tasks: [...]`

Optional root fields:

- `profiles: ["personal", "work"]`
- `variables: { Name: { prompt: "...", default: "..." } }`

Task fields:

- `action` (required)
- `args` (action-specific)
- `when` (optional expression string in `${ ... }` form)

`when` examples:

```yaml
when: ${ os == "darwin" }
when: ${ profile in ["work", "personal"] }
when: ${ exists(home + "/.config") }
```

`args` support full expressions and interpolation:

```yaml
args:
  - ${ home }/.config/app
  - ${ vars.repo_root }/templates
```

## Supported Actions

- `dir.create`
- `symlink.create`
- `template.render`
- `pkg.install`
- `pkg-manager.install`
- `mise.use`
- `git.config`
- `set.darwin.defaults`

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

## Validation and Tests

```bash
go test ./...
```

Schema file: `schema.json`
