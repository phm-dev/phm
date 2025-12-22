# PHM - PHP Manager for macOS

> **Disclaimer:** This software is provided "as is", without warranty of any kind, express or implied. The authors are not responsible for any damages or issues arising from the use of this software. Use at your own risk.

## Installation

```bash
curl -fsSL https://raw.githubusercontent.com/phm-dev/phm/main/scripts/install-phm.sh | bash
```

After installation, add to your shell profile (`~/.zshrc` or `~/.bashrc`):

```bash
export PATH="/opt/php/bin:$PATH"
```

## Quick Start

```bash
# Install PHP 8.5 with extensions (auto-syncs package index)
phm install php8.5-cli php8.5-fpm php8.5-redis

# Set as default version
phm use 8.5

# Verify
php -v

# Interactive mode (wizard)
phm ui
```

## Commands

```bash
phm install <package>    # Install packages
phm remove <package>     # Remove packages
phm upgrade              # Upgrade all packages
phm list                 # List available packages
phm search <query>       # Search packages
phm info <package>       # Show package details
phm use <version>        # Set default PHP version
phm fpm start|stop|...   # Manage PHP-FPM service
phm ext enable|disable   # Manage extensions
phm ui                   # Interactive wizard
```

**[Full Commands Reference](docs/commands.md)** - detailed documentation for all commands with examples.

---

# The Story Behind PHM

**Languages:**
- English (this file)
- [Polski](README.pl.md)

> *In the age of endless recompilation,*
> *when developers burned CPU cycles like incense,*
> *and dependency chains grew longer than elven genealogies —*
> *a different path was needed.*

---

## Realm Scope

PHM **runs exclusively on macOS**.

Not because other worlds are lesser —
but because this story belongs to a **specific realm**, where:

- Homebrew became the de-facto standard,
- recompiling PHP on laptops is considered normal,
- binary PHP packages simply **do not exist**.

PHM was forged to solve a **real macOS problem**:
the absence of a simple, system-level way to install PHP like this:

```bash
apt install php8.5-cli
```

Linux has its repositories.
Debian has Ondřej.
macOS had only the forges.

PHM fills that void.

---

## The Age of Darkness

Once, in the lands of macOS and Linux alike,
developers compiled PHP **again and again**.

On laptops.
On CI runners.
On build servers.
On machines that only wanted to run `php -v`.

Each day:

- the same sources,
- the same flags,
- the same extensions,
- the same errors,
- the same wasted hours,
- the same CO₂ rising silently from datacenters and fans.

> *Ten machines,*
> *ten builds,*
> *ten slightly different binaries,*
> *none of them truly reproducible.*

This was the **Age of Dependency Chaos**.

Homebrew spells conflicted.
ASDF charms broke silently.
phpenv grimoires rotted with outdated incantations.
And every developer paid the price —
in time, in sanity, in watts.

---

## The Curse of Recompilation

PHP is not light magic.

It pulls with it:

- OpenSSL
- ICU
- libxml
- libzip
- rabbitmq-c
- zlib
- iconv
- and countless others

To compile PHP once is acceptable.
To compile it **everywhere** is madness.

Yet the world accepted this madness as normal.

> *"Just rebuild it locally."*
> *"Just use brew."*
> *"Just try again."*

And so the forges burned.

---

## The Light from the North

In the Debian realms, a different pattern emerged.

A quiet master-smith named **Ondřej**
forged PHP **once** —
and shared the artifacts with the world.

Not spells.
Not source trees.
**Packages.**

Reproducible.
Predictable.
Installable.

```bash
apt install php8.2-cli
apt install php8.2-fpm
apt install php8.2-redis
```

No rebuilds.
No surprises.
No wasted fire.

> *One build.*
> *Thousands of installs.*
> *A sane world.*

---

## PHM Follows the Ancient Pattern

**PHM** is our answer for the modern realms.

Not a version manager.
Not a build system.
Not another dependency illusion.

PHM is a **package manager for PHP**, inspired by the ancient and powerful pattern of Ondřej.

### With PHM, you install **only what you need**:

```bash
phm install php8.5-cli
phm install php8.5-fpm
phm install php8.5-redis
```

Nothing more.
Nothing less.

Each package is:

- precompiled
- architecture-specific
- ABI-correct
- dependency-contained

No Homebrew.
No local compilation.
No hidden magic.

---

## What PHM Is

- A **binary package manager** for PHP
- A tool that installs **prebuilt PHP components**
- A system that respects:
  - CPU architectures
  - ABI stability
  - deterministic builds
- A way to stop recompiling PHP on every machine on Earth

---

## What PHM Is Not

- not phpenv
- not asdf
- not brew formulas
- not "just build it locally"

PHM does **not** pretend compilation is free.
PHM does **not** pretend developers have infinite time.
PHM does **not** pretend CO₂ is imaginary.

---

## The True Cost of Builds

Every unnecessary compilation costs:

- electricity
- cooling
- CPU lifespan
- developer focus
- planetary resources

Multiply that by:

- CI pipelines
- laptops
- teams
- companies
- years

> *The cost is real — even if your terminal stays silent.*

PHM exists to **end the waste**.

---

## Available Packages

### Core Packages (per PHP version)

| Package | Description |
|---------|-------------|
| `php8.5-common` | Shared files, php.ini |
| `php8.5-cli` | Command-line interpreter |
| `php8.5-fpm` | FastCGI Process Manager |
| `php8.5-cgi` | CGI binary |

### Extensions

| Package | Description |
|---------|-------------|
| `php8.5-opcache` | OPcache |
| `php8.5-redis` | Redis client |
| `php8.5-igbinary` | Binary serializer |
| `php8.5-mongodb` | MongoDB driver |
| `php8.5-amqp` | RabbitMQ client |
| `php8.5-xdebug` | Debugger |
| `php8.5-pcov` | Code coverage |
| `php8.5-memcached` | Memcached client |

---

## Links

- **PHM CLI**: https://github.com/phm-dev/phm
- **PHP Packages**: https://github.com/phm-dev/php-packages

---

> *Power does not come from endless recompilation.*
> *Power comes from restraint.*
> *From precision.*
> *From building once — and sharing wisely.*

PHM walks the old path.

And the forges may finally cool.

---

**PHM — Package PHP once. Install it everywhere.**
Inspired by the pattern of **Ondřej Surý**.
Forged for the modern dark realms.
