# Ground truth: NOVUS Installer implementation

**Status:** internal static audit.
**Observed branch:** `main`, commit `6a0d4ce`, with user changes in `internal/orchestrator/runner.go` and its test.
**Scope:** CLI, preflight, embedded HTTP server, async Runner, rollback and host-side commands.

## CLI and lifecycle

`cmd/installer/main.go` accepts exactly three flags:

| Flag | Implemented behavior |
| --- | --- |
| `--dev` | Converts OS, RAM, disk and port preflight failures into warnings; root remains mandatory |
| `--dry-run` | Causes Runner command execution and cleanup helpers to simulate rather than mutate host state |
| `--version` | Prints build version and exits |

Without `--version`, startup creates a signal-cancelled root context, runs preflight inside a **one-second** timeout, generates a **16-byte random token encoded as 32 hex characters**, then starts the temporary HTTP server on `:8080`. The server is not bound to loopback only.

Preflight failure prevents server startup. The program prints a fallback banner and exits through `log.Fatalf`; it does not expose an interactive UI that can fix the failed preflight.

## Preflight contract

| Check | Strict behavior | `--dev` behavior | Evidence |
| --- | --- | --- | --- |
| Effective UID | Must be root | Still fatal | `internal/preflight/preflight.go:118-124` |
| OS | Ubuntu `22.04`, `24.04`, `26.04`, `26.10`; Debian `12`, `13` | Warning | `preflight.go:21-32,131-145` |
| RAM | At least 2 GiB | Warning | `preflight.go:147-166` |
| Free disk on `/` | At least 10 GiB | Warning | `preflight.go:168-184` |
| TCP ports | Only `80` and `443` are bound-tested | Warning | `preflight.go:193-198` |
| Tools | `mysqldump`, `ssh`, `sudo`, `jq`, `openssl` | Warning in all modes | `preflight.go:186-190,327-333` |

The checked-in `docs/ARCHITECTURE.md` claim that `8080` and `9443` are preflight-gated. That is not true in the current source: only 80 and 443 are checked, even though the installer later binds 8080 and deployment rules open 9443.

## HTTP and WebSocket surface

| Endpoint | Authorization | Behavior |
| --- | --- | --- |
| `GET /?token=<token>` | Initial query token or authorized cookie | Serves embedded SPA and sets `HttpOnly`, `SameSite=Strict`, one-hour cookie |
| `GET /api/locales/*` | Cookie only | Embedded locale retrieval |
| `GET /api/stream` | Cookie only | WebSocket install status and binary PTY frames |
| `POST /api/setup` | Cookie only | Strict JSON decode (`DisallowUnknownFields`), validates setup request, starts Runner asynchronously and returns `{ "status": "installing" }` |

The session cookie has no `Secure` attribute in the current server implementation. The server uses header-read timeout 5 seconds and idle timeout 30 seconds. The one-time URL token has 128 bits of entropy, not the 256 bits stated by the old architecture document.

## Environment registry

| Variable | Type/default | Where used | Criticality |
| --- | --- | --- | --- |
| `NOVUS_INSTALLER_PANEL_RELEASE_URL` | URL; defaults to public panel release asset | `resolvePanelReleaseURL()` | Feature-critical for Panel archive acquisition |
| `NOVUS_INSTALLER_AGENT_BINARY_URL` | URL; architecture-specific GitHub release defaults | `resolveAgentBinaryURL()` | Feature-critical for Agent binary acquisition |
| `NOVUS_INSTALLER_MODULE_REGISTRY_URL` | URL; defaults to public modules registry index | `renderPanelEnvironment()` | Feature-critical for Module Registry bootstrap in panel |
| `NOVUS_INSTALLER_MODULE_BOOTSTRAP_POOL` | CSV list (or `all`/`*`) | `renderPanelEnvironment()` | Governs default module bootstrap allowlist at first start |

No environment variable controls listen address, preflight deadline, token length, Web cookie flags, or per-step deadline in this checkout.

## Async pipeline and failure behavior

`POST /api/setup` validates inputs synchronously and calls `Runner.Start()`. The runner marks itself active, spawns `run()` in a goroutine and the HTTP response remains `installing`; later errors are delivered through text stream messages and audit records.

| Condition | Actual behavior |
| --- | --- |
| Invalid request/domain/platform before steps | Emits an `error` status/frame and returns; no install steps run |
| Step error | Emits error, records audit event, then calls rollback for completed steps |
| Parent context cancellation | Records cancellation, emits error, then calls rollback |
| Rollback step error | Logs/audits failure and continues reverse sweep |
| Completed steps with no rollback handler | `rollback()` returns without soft/hard cleanup if no completed step has a rollback function |
| Soft cleanup | Runs on rollback after at least one rollback handler; stops services, removes installer artifacts and attempts to drop created DBs; preserves system packages |
| Hard cleanup | Runs only in `--dev`; purges packages and autoremove is best effort |
| Dry run | Command execution/removal paths report simulated work and do not mutate host state |
| Success | Emits `finish`, calls self-destruct policy, then invokes `OnComplete`; main schedules server stop after 3 seconds |
| Failure after async start | `OnComplete` is not called, so the temporary server remains until signal/shutdown |

Every install step gets `defaultPTYStepTimeout`; the outer Runner error path calls rollback with the same parent context. A cancelled parent context can therefore limit cleanup operations as well.

## System and privilege surface

The installer is root-only and deliberately performs host administration. It renders shell command strings and executes them via `bash -lc` (`internal/orchestrator/runner.go:1114`). Major effects include:

| Domain | Commands/files affected |
| --- | --- |
| Package/repository setup | `apt-get`, repository setup via curl/gpg, package install/purge |
| Service manager | `systemctl enable/start/stop/restart/daemon-reload/reset-failed` for Nginx, MariaDB, PHP-FPM, Agent, Redis, Supervisor, InfluxDB, Fail2Ban |
| Firewall | UFW defaults and ports 22, 80, 443, 8080, 9443 |
| Database | MariaDB/MySQL SQL bootstrap and rollback drop statements, temporary `/tmp/novus_db_setup.sql` |
| Artifacts | Agent download to `/tmp/novus-agent`, install to `/usr/local/bin/novus-agent`; Panel archive and deployment artifacts; canonical `/etc/novus` state |
| Web/security config | Nginx site files, `/etc/sudoers.d`, `/etc/cron.d`, Supervisor configs, PHP-FPM drop-in |
| Secrets | Creates/manages `/etc/novus/secrets` and Panel environment material |

`steps_system.go` contains numerous `|| true` continuations. A command can therefore be masked at shell level; later steps or health checks are the practical detection point. Documentation must not claim every dependency command fails fast.

## Documentation corrections required

1. Correct token size from 32 bytes to 16 bytes, and correct preflight port scope from `80/443/8080/9443` to current `80/443`.
2. Document that token cookie is `HttpOnly` and `SameSite=Strict` but not `Secure` in current source.
3. Distinguish synchronous request acceptance from asynchronous install success. `200 installing` is not a completed installation.
4. Document rollback as best effort, conditional on completed rollback-capable steps, and not as a transaction.
5. Keep host command inventory in internal documentation; public installation material should describe required privileges without exposing secrets or rendering sensitive setup input.