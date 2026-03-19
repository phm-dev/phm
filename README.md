# PHM — PHP Manager for macOS

**PHM (PHP Manager)** is a binary package manager for PHP on macOS. It provides precompiled PHP packages that install in seconds — no Homebrew, no MacPorts, no local compilation required.

PHM follows the Debian/Ubuntu model: PHP and its extensions are built once on CI, distributed as binary packages, and installed instantly on any Mac.

> **Disclaimer:** This software is provided "as is", without warranty of any kind, express or implied.

## Installation

```bash
curl -fsSL https://raw.githubusercontent.com/phm-dev/phm/main/scripts/install-phm.sh | bash
```

Add to your shell profile (`~/.zshrc` or `~/.bashrc`):

```bash
export PATH="/opt/php/bin:$PATH"
```

## Quick Start

```bash
# Install PHP 8.5 with extensions
phm install php8.5-cli php8.5-fpm php8.5-redis

# Set as default version
phm use 8.5

# Install developer tools
phm install composer symfony

# Verify
php -v
```

## Why PHM

On Linux, `apt install php8.5-cli php8.5-redis` takes seconds. On macOS, you're stuck with Homebrew (compiles from source, 10+ minutes, breaks on updates) or PECL for extensions (requires Xcode, build tools, manual php.ini configuration, and can cause macOS code signing crashes).

PHM gives macOS the same experience as `apt`:

- **Instant installation** — precompiled binaries, zero compilation
- **40+ extensions** as packages — Redis, Xdebug, ImageMagick, MongoDB, and more
- **Multiple PHP versions** — 8.1, 8.2, 8.3, 8.4, 8.5 side by side
- **Built-in tools** — `phm install composer symfony phpstan php-cs-fixer`
- **PHP-FPM management** — `phm fpm start|stop|restart`
- **Per-project PHP versions** — via Symfony CLI + `.php-version`
- **macOS native** — Apple Silicon and Intel, macOS 13+

## Commands

```bash
phm install <package>         # Install packages or tools
phm remove <package>          # Remove packages or tools
phm upgrade                   # Upgrade all packages
phm list                      # List installed packages
phm search <query>            # Search packages
phm info <package>            # Show package details
phm use <version>             # Set default PHP version
phm fpm start|stop|restart    # Manage PHP-FPM
phm ext enable|disable <ext>  # Manage extensions
phm self-update               # Update PHM itself
```

## Available Tools

PHM has a built-in tool manager:

```bash
phm install composer        # Dependency Manager for PHP
phm install symfony         # Symfony CLI
phm install phpstan         # Static analysis
phm install php-cs-fixer    # Coding standards
phm install psalm           # Static analysis (Vimeo)
phm install laravel         # Laravel installer
phm install deployer        # Deployment tool
phm install castor          # Task runner
```

## Version Pinning

```bash
# Track latest 8.5.x (recommended) — receives patch updates
phm install php8.5-cli

# Pin specific version — stays on exact version
phm install php8.5.1-cli

# Both can coexist
phm use 8.5      # latest
phm use 8.5.1    # pinned
```

## Documentation

Full documentation: **[phm-dev.github.io](https://phm-dev.github.io)**

- [What is PHM](https://phm-dev.github.io/what-is-phm/) — problem, solution, how it works
- [Installation](https://phm-dev.github.io/installation/) — detailed setup guide
- [Developer Tools](https://phm-dev.github.io/tools/) — Composer, Symfony CLI, PHPStan
- [Version Switching](https://phm-dev.github.io/version-switching/) — per-project PHP versions
- [Available Packages](https://phm-dev.github.io/packages/) — full package list
- [PHM vs Alternatives](https://phm-dev.github.io/phm-vs-alternatives/) — comparison with Homebrew, Docker, phpbrew
- [FAQ](https://phm-dev.github.io/faq/) — common questions

## Links

- [PHM CLI](https://github.com/phm-dev/phm) — this repository
- [PHP Packages](https://github.com/phm-dev/php-packages) — build scripts and package repository
- [Documentation](https://phm-dev.github.io) — full docs

---

## The Story Behind PHM

> *In the age of endless recompilation, when developers burned CPU cycles like incense and dependency chains grew longer than elven genealogies — a different path was needed.*

In the Debian realms, a quiet master-smith named **Ondřej** forged PHP **once** — and shared the artifacts with the world. Not spells. Not source trees. **Packages.** Reproducible. Predictable. Installable.

```bash
apt install php8.5-cli php8.5-fpm php8.5-redis
```

macOS had no such path. Homebrew spells conflicted. phpenv grimoires rotted with outdated incantations. And every developer paid the price — in time, in sanity, in watts.

PHM follows the ancient pattern of Ondřej. Build once. Share wisely. Install instantly.

> *Power does not come from endless recompilation.*
> *It comes from building once — and sharing wisely.*

**PHM — Package PHP once. Install it everywhere.**
