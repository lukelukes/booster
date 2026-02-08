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

## Examples

```yaml
${ os == "darwin" }
${ profile in ["work", "personal"] }
${ exists(home + "/.config") }
${ installed("git") }
${ default(vars.name, "fallback") }
${ "b" in ["a", "b", "c"] }
${ "hello" contains "ell" }
```
