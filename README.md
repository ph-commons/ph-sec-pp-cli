# Philippines SEC CLI (`ph-sec-pp-cli`)

[![CI](https://github.com/ph-commons/ph-sec-pp-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/ph-commons/ph-sec-pp-cli/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

**Philippines SEC free SEC Number lookup — by registration number only.**  
**Not name search. Not GIS/AFS. Not US EDGAR.**

---

## ⚠ Free still requires a token

| Myth | Fact |
|------|------|
| “Free package = no login / no API key” | **False.** Free means **no CIL package fee**, not unauthenticated access |
| “I can call the gateway without credentials” | **False.** WSO2 returns **HTTP 401** `Missing Credentials` without a bearer |
| “`PH_SEC_TOKEN` is only for paid features” | **False.** **Every live call** needs `PH_SEC_TOKEN` — including free status lookup |

**Required for live use:**

1. Credentialed eSECURE account → [SEC API Marketplace](https://portal.sec.gov.ph/)
2. Subscribe to the **Free SEC Number** package (not paid CompanyInformationLookup for this CLI)
3. Generate an OAuth2 access token (Try Out → Get Test Key, or Applications → Production Keys)
4. Export it:

```bash
export PH_SEC_TOKEN='your_bearer_access_token'
# or: ph-sec-pp-cli auth set-token 'your_bearer_access_token'
```

Without step 4, live commands fail with auth errors (typically **exit code 4**). That is expected, not a CLI bug.

**Offline after sync:** `company show`, `search`, and `sql` can run without a token **only if** the local DB already has rows from a previous authenticated fetch.

---

## Scope (Path A / free-only)

| | |
|---|---|
| **What it is** | Registration **status by a known SEC number** (one free Marketplace product) |
| **What you need for live** | Free package subscription + **`PH_SEC_TOKEN`** (Bearer) |
| **What it is not** | Name search, addresses, AFS, GIS, eSEARCH PDFs, US SEC EDGAR, PSE quotes |
| **Quota** | Free package is roughly **~10 API calls/day** (portal-stated; re-check). Prefer offline after first fetch |
| **Paid CIL** | Out of this binary (future Path B if you subscribe) |
| **PSE stocks** | Use `pse-edge-pp-cli` |

**Primary live command:**

```bash
ph-sec-pp-cli companies --sec-no <SEC_NO> --json
```

You must already know the SEC number. This CLI cannot look up companies by trade name.

Created by [@ngpestelos](https://github.com/ngpestelos) (Nestor G Pestelos Jr).

## Install

```bash
# Go 1.26.5+
go install github.com/ph-commons/ph-sec-pp-cli/cmd/ph-sec-pp-cli@latest

# Or clone and build
git clone https://github.com/ph-commons/ph-sec-pp-cli.git
cd ph-sec-pp-cli
go build -o ./ph-sec-pp-cli ./cmd/ph-sec-pp-cli
```

Prebuilt binaries (when releases exist): [GitHub Releases](https://github.com/ph-commons/ph-sec-pp-cli/releases).

## Authentication (required for live)

**There is no anonymous public API.** Free SEC Number still uses WSO2 OAuth2 Bearer.

| Step | Action |
|------|--------|
| 1 | Credentialed eSECURE account (eKYC / MC 10 s.2024) |
| 2 | Sign in at [portal.sec.gov.ph](https://portal.sec.gov.ph/) |
| 3 | Subscribe to **Free** SEC Number package |
| 4 | Create token (Try Out → OAuth2 → Get Test Key, **or** Production Keys → Consumer Key/Secret + token URL) |
| 5 | `export PH_SEC_TOKEN=<bearer>` |

Gateway base: `https://gwwso2.sec.gov.ph/secnumber/1.0.0`  
Header: `Authorization: Bearer <token>`

This CLI does **not** implement paid CompanyInformationLookup auth.

| Env var | Required for live | Description |
|---------|-------------------|-------------|
| `PH_SEC_TOKEN` | **Yes** | Marketplace OAuth bearer (Free package is enough for this CLI) |

Optional: `ph-sec-pp-cli auth set-token <token>` stores the credential in the data dir; `doctor` reports whether it is configured.

## Quick Start

```bash
# 1) Binary present?
ph-sec-pp-cli doctor --dry-run

# 2) REQUIRED for live — free package still needs OAuth
export PH_SEC_TOKEN='your_marketplace_bearer_token'

# 3) Confirm the CLI sees the token (before spending free quota)
ph-sec-pp-cli doctor

# 4) Live free-product lookup (1 free call; known SEC number only)
ph-sec-pp-cli companies --sec-no A199600179 --json

# 5) Cache-first re-read (no network unless --refresh)
ph-sec-pp-cli company show --sec-no A199600179 --json
```

Skip step 2 → steps 4–5 fail with 401 / auth exit. That confirms free ≠ open.

## Commands that matter

| Command | Needs token? | Notes |
|---------|--------------|--------|
| `companies --sec-no <N>` | **Yes (live)** | Free product status lookup |
| `company show --sec-no <N>` | No if cached; **Yes** with `--refresh` | Cache-first |
| `search <q>` | No (local only) | Only companies already synced |
| `sql --query "SELECT …"` | No (local only) | SELECT against local mirror |
| `doctor` | No | Shows missing `PH_SEC_TOKEN` |
| `which` | No | Free-only boundary hints |

```bash
ph-sec-pp-cli companies --sec-no A199600179 --json
ph-sec-pp-cli company show --sec-no A199600179 --json
ph-sec-pp-cli company show --sec-no A199600179 --refresh --json   # burns free quota
ph-sec-pp-cli search corporation --json
ph-sec-pp-cli sql --query "SELECT sec_no, company_name, status FROM companies LIMIT 10" --json
ph-sec-pp-cli doctor --json
ph-sec-pp-cli which "name search" --json
```

## Output formats

```bash
ph-sec-pp-cli companies --sec-no A199600179              # table in TTY
ph-sec-pp-cli companies --sec-no A199600179 --json
ph-sec-pp-cli companies --sec-no A199600179 --json --select company_name,status
ph-sec-pp-cli companies --sec-no A199600179 --dry-run    # no network
ph-sec-pp-cli companies --sec-no A199600179 --agent      # JSON + compact + no prompts
```

Exit codes: `0` success, `2` usage, `3` not found, `4` **auth**, `5` API, `7` rate limited, `10` config.

## Paths & config

| Kind | Env override | Default idea |
|------|--------------|--------------|
| config | `PH_SEC_CONFIG_DIR` | `~/.config/ph-sec-pp-cli` |
| data (DB, credentials) | `PH_SEC_DATA_DIR` | `~/.local/share/ph-sec-pp-cli` |
| state | `PH_SEC_STATE_DIR` | XDG state |
| cache | `PH_SEC_CACHE_DIR` | XDG cache |

Single root for agents/containers:

```bash
export PH_SEC_HOME=/srv/ph-sec
ph-sec-pp-cli doctor
```

## Agent notes

- Non-interactive; prefer `--json` / `--agent`
- **Always set `PH_SEC_TOKEN` for live** — free product is not public
- Do not invent name search, GIS, or AFS commands
- After one live fetch, prefer local `company show` / `search` / `sql` to save ~10/day free quota
- Self-learning (`teach` / `recall`) is available; disable with `--no-learn` or `PH_SEC_NO_LEARN=true`

## Troubleshooting

| Symptom | Fix |
|---------|-----|
| Exit 4 / 401 Missing Credentials | Set `PH_SEC_TOKEN` to Free-package OAuth bearer; free is not anonymous |
| “Want to search by company name” | Not in this CLI — paid CIL or portal |
| GIS / AFS / directors | Out of scope — paid CIL / eSEARCH |
| Confused with US SEC | Use `edgar` / `sec-edgar`, not `ph-sec` |
| Quota exhausted | Wait for daily free reset; use offline cache |

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press). Path A free-only. Repository: [ph-commons/ph-sec-pp-cli](https://github.com/ph-commons/ph-sec-pp-cli).

## License

Apache License 2.0. See [LICENSE](LICENSE) and [NOTICE](NOTICE).

- CLI copyright: Nestor G Pestelos Jr and contributors
- Generated with [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press) (MIT, separate)
