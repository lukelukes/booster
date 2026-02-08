# booster

Declarative machine bootstrap tool driven by YAML tasks.
Linux and macOS.

## Quick start

```bash
make build
./out/booster run --config ./bootstrap.yaml
```

## Documentation

- **[Tutorial: Bootstrap your first machine](docs/tutorial/bootstrap-your-first-machine.md)** — Learn booster by building a config from scratch
- **How-to guides** — Solve specific problems:
  - [Install and build](docs/howto/install-and-build.md)
  - [Write a config](docs/howto/write-a-config.md)
  - [Use profiles](docs/howto/use-profiles.md)
  - [Use variables](docs/howto/use-variables.md)
  - [Add conditional tasks](docs/howto/add-conditional-tasks.md)
- **Reference** — Look up specifics:
  - [CLI](docs/reference/cli.md)
  - [Config schema](docs/reference/config.md)
  - [Actions](docs/reference/actions.md)
  - [Expressions](docs/reference/expressions.md)
- **[Design and rationale](docs/explanation/design.md)** — Why booster works the way it does
