# Expressions Reference

Expressions use `${ ... }` delimiters and are powered by [expr-lang/expr](https://expr-lang.org/).

## Syntax

### Full expression

The entire string value is a single `${ expr }` block. Returns the expression's native type (bool, string, int, etc.).

```yaml
when: ${ os == "darwin" }
```

### Interpolation

Expressions embedded within a larger string. Always resolves to string.

```yaml
args:
  - ${ home }/.config/nvim
```

## Context Variables

| Variable | Type | Description |
|---|---|---|
| `os` | string | Detected OS. On macOS: `"darwin"`. On Linux: distro ID from `/etc/os-release` (e.g., `"arch"`, `"ubuntu"`, `"fedora"`). Falls back to `runtime.GOOS`. |
| `os_family` | string | Broad platform from `runtime.GOOS`: `"linux"` or `"darwin"`. Use it to branch on the platform regardless of distro. |
| `arch` | string | CPU architecture from `runtime.GOARCH` (e.g., `"amd64"`, `"arm64"`). |
| `home` | string | Value of `$HOME` environment variable. |
| `profile` | string | Selected profile from `--profile` flag. Empty string if no profiles defined. |
| `env` | map[string]string | All environment variables. Access as `env.KEY`. |
| `vars` | map[string]any | User-defined variables from config. Access as `vars.Name`. |

## Functions

| Signature | Return | Description |
|---|---|---|
| `exists(path string)` | bool | Returns true if the file or directory at path exists. Supports `~` expansion. |
| `which(name string)` | string | Returns the full path of the named executable, or empty string if not found. |
| `installed(name string)` | bool | Returns true if the named executable is found in PATH. |
| `default(value any, fallback any)` | any | Returns fallback if value is nil or empty string. |
| `expand(path string)` | string | Expands `~` to home directory and environment variables. |
| `hasSubstr(haystack string, needle string)` | bool | Returns true if haystack contains needle. |
| `join(list []any, separator string)` | string | Joins list elements with separator. |

## Operators

| Operator | Description |
|---|---|
| `==`, `!=`, `<`, `>`, `<=`, `>=` | Comparison |
| `and`, `or`, `not` | Logical |
| `in` | List membership or map key existence |
| `contains` | String substring check |
| `+`, `-`, `*`, `/`, `%` | Arithmetic |
| `+` | String concatenation |

## The `when` Field

The `when` field requires the expression to evaluate to bool.

## Picking a value with `match`

A `match` block picks one value out of several. Use it anywhere in task `args` or as a variable `default`.

```yaml
- action: pkg.install
  args:
    match:
      on: ${ os }
      cases:
        arch: [git-delta, ghostty]
        darwin:
          - packages: [git-delta, gh]
          - casks: [ghostty]
      default: []
```

| Field | Required | Description |
|---|---|---|
| `on` | Yes | A string or expression. It is evaluated once. |
| `cases` | Yes | A mapping from case value to branch. The result of `on` is turned into a string and compared exactly with each key. |
| `default` | No | The branch to use when no case matches. `default: ~` and `default: []` both count as a default. |

The block is replaced by the chosen branch, so the action sees the branch as if you had written it there. A branch can be a string, a list or a mapping, and it can hold more `match` blocks or `${ }` expressions. Branches that are not chosen are never evaluated.

A mapping is a `match` block only when `match` is its only key. If no case matches and there is no `default`, booster stops before running anything and reports the task and the path, for example `task 3 (pkg.install): args[1].match: no case for "ubuntu" and no default`.

For two branches a ternary is fine: `${ os == "darwin" ? "brew" : "pacman" }`. For three or more, use `match` instead of nesting ternaries.

## Examples

```yaml
${ os == "darwin" }
${ os_family == "linux" }
${ profile in ["work", "personal"] }
${ exists(home + "/.config") }
${ installed("git") }
${ default(vars.name, "fallback") }
${ "b" in ["a", "b", "c"] }
${ "hello" contains "ell" }
```
