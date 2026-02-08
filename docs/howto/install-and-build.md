# How to install and build booster

## Prerequisites

- Go 1.25 or later
- Make
- git

## Steps

1. Clone the repository:

```bash
git clone https://github.com/lukelukes/booster.git
cd booster
```

2. Build the binary:

```bash
make build
```

The binary is written to `out/booster`.

3. Verify the build:

```bash
out/booster version
```

4. Optionally, install to your PATH:

```bash
make install
```

This copies the binary to `~/.local/bin/booster`.

## Troubleshooting

**`go: go.mod requires go >= 1.25`**

Your Go version is too old. Run `go version` and upgrade if below 1.25.

**`booster: command not found` after `make install`**

`~/.local/bin` is not in your PATH. Add it:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

Add that line to your shell profile (`~/.bashrc`, `~/.zshrc`, etc.) to persist it.

**`make: command not found`**

Install Make via your package manager (`sudo pacman -S make`, `sudo apt install make`, `brew install make`).
