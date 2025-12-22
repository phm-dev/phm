# PHM Commands Reference

Complete reference for all PHM commands.

## Table of Contents

- [Global Flags](#global-flags)
- [Package Management](#package-management)
  - [install](#install)
  - [remove](#remove)
  - [upgrade](#upgrade)
  - [list](#list)
  - [search](#search)
  - [info](#info)
- [Version Management](#version-management)
  - [use](#use)
- [Extension Management](#extension-management)
  - [ext](#ext)
- [PHP-FPM Management](#php-fpm-management)
  - [fpm](#fpm)
- [Interactive Mode](#interactive-mode)
  - [ui](#ui)
- [Configuration](#configuration)
  - [config](#config)
- [Shell Completion](#shell-completion)
  - [completion](#completion)
- [Destructive Operations](#destructive-operations)
  - [destruct](#destruct)

---

## Global Flags

These flags are available for all commands:

| Flag | Description |
|------|-------------|
| `--debug` | Enable debug output |
| `--offline` | Use offline mode (local repository) |
| `--repo <path>` | Path to local repository (implies --offline) |
| `-h, --help` | Help for any command |
| `-v, --version` | Show PHM version |

---

## Package Management

### install

Install one or more packages.

```bash
phm install [packages...] [flags]
```

**Aliases:** `i`

**Flags:**

| Flag | Description |
|------|-------------|
| `-f, --force` | Force reinstall even if package is already installed |

**Features:**

- **Auto-sync:** Package index is automatically synced before installation
- **Auto-upgrade:** When installing an extension (e.g., `php8.5-redis`), all other installed packages of the same PHP version are automatically upgraded first to ensure compatibility
- **Progress bar:** Downloads show a progress bar with speed and percentage

**Examples:**

```bash
# Install PHP CLI
phm install php8.5-cli

# Install multiple packages
phm install php8.5-cli php8.5-fpm php8.5-redis

# Force reinstall
phm install -f php8.5-cli

# Install extension (auto-upgrades other php8.5-* packages first)
phm install php8.5-redis
```

---

### remove

Remove one or more packages.

```bash
phm remove [packages...] [flags]
```

**Aliases:** `rm`, `uninstall`

**Examples:**

```bash
# Remove a single package
phm remove php8.5-redis

# Remove multiple packages
phm remove php8.5-xdebug php8.5-pcov
```

---

### upgrade

Upgrade installed packages to their latest versions.

```bash
phm upgrade [packages...] [flags]
```

**Examples:**

```bash
# Upgrade all installed packages
phm upgrade

# Upgrade specific packages
phm upgrade php8.5-cli php8.5-redis
```

---

### list

List packages.

```bash
phm list [pattern] [flags]
```

**Aliases:** `ls`

**Flags:**

| Flag | Description |
|------|-------------|
| `-i, --installed` | Show installed packages (default: true) |
| `-a, --available` | Show available packages |

**Examples:**

```bash
# List installed packages
phm list

# List available packages
phm list -a

# List packages matching pattern
phm list php8.5

# List available packages matching pattern
phm list -a redis
```

---

### search

Search for packages by name or description.

```bash
phm search <query> [flags]
```

**Aliases:** `s`

**Examples:**

```bash
# Search for redis packages
phm search redis

# Search for PHP 8.5 packages
phm search php8.5
```

---

### info

Show detailed information about a package.

```bash
phm info <package> [flags]
```

**Aliases:** `show`

**Examples:**

```bash
# Show info about a package
phm info php8.5-cli

# Show info about an extension
phm info php8.5-redis
```

---

## Version Management

### use

Set the default PHP version. Creates symlinks in `/opt/php/bin/` for the selected version.

```bash
phm use <version> [flags]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--system` | Also create symlinks in `/usr/local/bin` (may conflict with Homebrew) |

**Examples:**

```bash
# Show current and available versions
phm use

# Set PHP 8.5 as default
phm use 8.5

# Set PHP 8.5 as default and create system-wide symlinks
phm use 8.5 --system
```

> **Note:** By default, symlinks are created only in `/opt/php/bin/`. Make sure this directory is in your PATH. Use `--system` to also create symlinks in `/usr/local/bin`, but be aware this may conflict with Homebrew PHP installations.

---

## Extension Management

### ext

Manage PHP extensions (enable/disable).

```bash
phm ext <action> [extension] [flags]
```

**Actions:**

| Action | Description |
|--------|-------------|
| `list` | List available extensions and their status |
| `enable <ext>` | Enable an extension |
| `disable <ext>` | Disable an extension |

**Flags:**

| Flag | Description |
|------|-------------|
| `--version <ver>` | PHP version (default: current default version) |

**Examples:**

```bash
# List all extensions for current PHP version
phm ext list

# List extensions for PHP 8.5
phm ext list --version=8.5

# Enable redis
phm ext enable redis

# Disable xdebug
phm ext disable xdebug

# Enable redis for specific PHP version
phm ext enable redis --version=8.4
```

---

## PHP-FPM Management

### fpm

Manage PHP-FPM services for different PHP versions.

```bash
phm fpm <action> [version] [flags]
```

**Actions:**

| Action | Description |
|--------|-------------|
| `status` | Show status of all PHP-FPM services |
| `start <version>` | Start PHP-FPM for a specific version |
| `stop <version>` | Stop PHP-FPM for a specific version |
| `restart <version>` | Restart PHP-FPM for a specific version |
| `reload <version>` | Reload PHP-FPM configuration |
| `enable <version>` | Enable PHP-FPM to start at boot |
| `disable <version>` | Disable PHP-FPM from starting at boot |

**Examples:**

```bash
# Show status of all PHP-FPM services
phm fpm status

# Start PHP-FPM for PHP 8.5
phm fpm start 8.5

# Stop PHP-FPM for PHP 8.4
phm fpm stop 8.4

# Restart PHP-FPM
phm fpm restart 8.5

# Reload configuration without restart
phm fpm reload 8.5

# Enable PHP-FPM to start at boot
phm fpm enable 8.5

# Disable PHP-FPM from starting at boot
phm fpm disable 8.5
```

---

## Interactive Mode

### ui

Launch the interactive TUI (Terminal User Interface) wizard.

```bash
phm ui [flags]
```

The TUI provides a user-friendly interface for:

- Installing PHP versions
- Selecting SAPIs (cli, fpm, cgi)
- Choosing extensions
- Managing installed PHP versions

**Examples:**

```bash
# Launch interactive wizard
phm ui
```

---

## Configuration

### config

Show current PHM configuration.

```bash
phm config [flags]
```

**Examples:**

```bash
# Show configuration
phm config
```

---

## Shell Completion

### completion

Generate shell autocompletion scripts.

```bash
phm completion <shell> [flags]
```

**Supported Shells:**

- `bash`
- `zsh`
- `fish`
- `powershell`

**Flags:**

| Flag | Description |
|------|-------------|
| `--no-descriptions` | Disable completion descriptions |

### Zsh Setup

```bash
# Enable completions (if not already enabled)
echo "autoload -U compinit; compinit" >> ~/.zshrc

# Load completions for current session
source <(phm completion zsh)

# Install permanently (macOS)
phm completion zsh > $(brew --prefix)/share/zsh/site-functions/_phm

# Install permanently (Linux)
phm completion zsh > "${fpath[1]}/_phm"
```

### Bash Setup

```bash
# Load completions for current session
source <(phm completion bash)

# Install permanently (macOS)
phm completion bash > $(brew --prefix)/etc/bash_completion.d/phm

# Install permanently (Linux)
phm completion bash > /etc/bash_completion.d/phm
```

---

## Destructive Operations

### destruct

Completely remove all PHP versions installed by PHM and all PHM data.

```bash
phm destruct [flags]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `-f, --force` | Skip confirmation prompt |

**This command will:**

- Stop all PHP-FPM services
- Remove all PHP installations from `/opt/php/`
- Remove PHM symlinks from `/opt/php/bin` and `/usr/local/bin`
- Remove LaunchDaemons for PHP-FPM
- Remove cache (`~/.cache/phm`)
- Remove installed packages database (`~/.local/share/phm`)
- Remove configuration (`~/.config/phm`)
- Remove PHP-FPM sockets and logs

> **Note:** This does NOT remove the `phm` binary itself.

**Examples:**

```bash
# Remove everything with confirmation
phm destruct

# Remove everything without confirmation
phm destruct --force
```

> **Warning:** This is a destructive operation. All PHP installations and configurations will be permanently deleted.
