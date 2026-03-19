# Changelog

## v0.6.0

### Security

- **Lock mechanism rewritten** — replaced PID-file lock with `syscall.Flock(LOCK_EX|LOCK_NB)` to eliminate TOCTOU race condition
- **Lock added to upgrade and remove** — all mutating operations now hold an exclusive lock
- **Self-update integrity** — SHA256 checksum verification of downloaded tarball; atomic binary replacement via temp+rename
- **Self-update path traversal fix** — replaced local `extractTarGz` with `tools.ExtractTarGz` which validates paths
- **HTTPS enforcement** — remote index fetch now rejects non-HTTPS URLs
- **Bounded reads** — `io.ReadAll` calls capped: 10MB for index, 1MB for pkginfo/config, 500MB for binaries
- **HTTP client timeout** — all HTTP calls now use a shared client with 60s timeout
- **Tar entry filtering** — only `TypeReg`/`TypeRegA` extracted; symlinks, hardlinks, device nodes silently skipped
- **Setuid/setgid bit stripping** — `header.Mode` masked with `& 0777` before `chmod` to prevent privilege escalation
- **Two-pass extraction** — `pkginfo.json` read and validated before any files are extracted; missing metadata = hard error
- **File size validation** — tar header size checked before extraction; `io.Copy` result verified against expected size
- **Package name sanitization** — `safeNameRegex` validates all package names before use in file paths or database writes
- **Version string validation** — `safeVersionRegex` rejects non-numeric versions in pkginfo.json
- **InstallSlot validation** — regex-checked to prevent path injection via crafted slot values
- **Install prefix consistency** — slot rewriting uses `m.installPrefix` instead of hardcoded `/opt/php/`
- **Remove path validation** — files outside install prefix (+ allowed system paths) are skipped with a warning
- **Directory cleanup bounded** — `rmdir` loop uses `filepath.Clean` and prefix check, cannot escape install prefix
- **SUDO_USER injection prevention** — username/group validated before config placeholder substitution
- **Atomic database writes** — package database uses write-to-temp + `os.Rename` to prevent corruption on crash
- **Lock path from installPrefix** — lock file location respects configured install prefix

### Fixed

- **sourceSlot for extensions** — derived from `PHPVersion` (not `Version`) so redis 6.1.0 correctly maps to PHP 8.5 slot
- **compareVersions with suffixes** — `stripPreRelease` handles segments like `0-beta1` or `1rc2`
- **FPM package installation** — `/Library/LaunchDaemons/` added to allowed system paths
- **Package names with `+`** — `dio0.3.0+pie` and similar names now accepted
- **Directory entries in tar** — `TypeDir` skipped before path validation to avoid false positives on parent dirs like `/opt/`
- **Corrupted DB entry warnings** — `LoadInstalled` now warns to stderr instead of silently skipping
- **Circular dependency detection** — `ResolveDependencies` detects and reports cycles
- **Empty directory cleanup** — `Remove` cleans up empty parent directories after file deletion
- **Precompiled regexps** — `expandMetaPackages` no longer recompiles regexps in a loop
- **Deduplicated download/extract** — removed local `downloadFile`/`extractTarGz` from main.go in favor of `tools.*`

### Added

- `internal/httputil/client.go` — shared HTTP client with 60s timeout
- `internal/pkg/lock.go` — `AcquireLock(lockDir)` with `syscall.Flock`
- `internal/tools/download.go` — `DownloadFile` with completeness check
- `internal/tools/extract.go` — `ExtractTarGz` with path traversal protection
