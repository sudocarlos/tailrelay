---
name: documentation
description: Updating all tailrelay documentation — README, CHANGELOG, release notes, AGENTS.md, SKILL.md files, webui/README.md, UI screenshots, and the Docusaurus docs site. Use when adding user-facing features, releasing a new version, updating component versions, or keeping the docs in sync with the codebase.
---

# Documentation

## Overview

tailrelay documentation spans these locations, which must stay consistent with each other and with the source code:

| Document | Audience | Update Trigger |
|----------|----------|---------------|
| `README.md` | End users | Features, Quick Start, version, links to docs site |
| `docs/openapi.yaml` | API consumers | Any handler/route change in `webui/internal/handlers/`, `webui/internal/web/server.go` |
| `website/` | End users & developers | Guides (getting started, auth, dev, troubleshooting); rendered API reference is generated from `docs/openapi.yaml` automatically |
| `docs/screenshots/` | End users | Any visible UI change (captures mirrored into `website/static/img/screenshots/`) |
| `CHANGELOG.md` | Users upgrading | Every release; every significant change |
| `webui/README.md` | Developers building from source | Web UI API, config, build changes |
| `AGENTS.md` | Coding agents | Skills table, file map, env vars, review SHAs |
| `.agents/skills/*.SKILL.md` | Coding agents | Component-level knowledge changes |

---

## 1. README.md

### Structure

1. Title + badges (Docker Pulls, GitHub Release, License)
2. Documentation site link
3. Table of Contents
4. Features bullet list + Web UI details (SPA features, auth & access)
5. Getting Started (Quick Start → Prerequisites → Tailscale Setup → StartOS Deployment)
6. Troubleshooting
7. Screenshots
8. Development (Local WebUI Dev → Building → Testing)
9. Documentation (link to the published Docusaurus site + API reference)
10. Contributing

### What to Update

**New feature added:**
- Add to the Features bullet list (top of file)
- Add to the Web UI features list (if it's a UI feature)
- Add or update the relevant Getting Started section
- If it changes the API, update `docs/openapi.yaml` — the README no longer
  duplicates endpoint documentation; the published site regenerates from
  the spec automatically

**Version bump:**
- Update the `[![GitHub Release](...)]` badge URL if needed (it auto-pulls from GitHub Releases, so the badge usually self-updates)
- Update any version-specific `docker pull` commands or example tags

**Testing section:**
Keep in sync with actual test commands. The current canonical test commands are:
```bash
cd webui && go test ./...          # Go unit tests
pytest tests/integration/ -v       # Integration tests
```

---

## 1b. docs/openapi.yaml + website/

`docs/openapi.yaml` is the single source of truth for the HTTP/JSON API —
grounded directly in the route registration (`webui/internal/web/server.go`)
and handler implementations (`webui/internal/handlers/`). It is **not**
hand-duplicated anywhere else; the README and the docs site both link to the
reference rendered from it.

**When a handler or route changes:**
- Update the corresponding path/operation in `docs/openapi.yaml` (request
  body, responses, status codes, error shapes)
- No other file needs touching — `website/` (via `docusaurus-openapi-docs`)
  generates MDX from the spec via `docusaurus gen-api-docs all`, and CI
  (`.github/workflows/docs.yml`) regenerates and redeploys the GitHub Pages
  site whenever `docs/openapi.yaml` changes

**When adding a new guide page:**
- Add a Markdown file under `website/docs/`
- Register it in `website/sidebars.ts`
- Verify locally: `cd website && npm run docusaurus gen-api-docs all && npm run build`

**Branding:** render the product name with
`<BrandName />` (`website/src/components/BrandName`) — solid "Tail" plus an
outlined "relay", matching `.brand-relay` in `webui/frontend/src/app.css`. It's
used by the navbar (via the swizzled `website/src/theme/Logo`), the homepage
hero, and the Introduction heading. A page needs the `.mdx` extension to import
it. Plain `tailrelay` stays lowercase in prose, `siteConfig.title`, and
metadata, matching the repo and Docker Hub names.

**Ownership split for guide content (Quick Start, Tailscale Setup,
Troubleshooting, etc.):** `website/docs/*.md` is the canonical, detailed
version. `README.md` keeps a condensed copy for GitHub browsing (Docker Hub,
`git clone`, no JS) — when one changes, check whether the other needs the
same fix. Prefer editing `website/docs/*.md` first, then trim the README's
copy to match rather than letting them diverge.

---

## 1c. Screenshots

`docs/screenshots/take-screenshots.mjs` captures every UI screenshot with
Playwright against the Vite dev server, mocking all API responses — no
container, no Tailscale, no backend. Sources live in `docs/screenshots/`; the
script mirrors each capture into `website/static/img/screenshots/` (referenced
by `website/docs/screenshots.mdx`). Commit both copies.

```bash
cd webui/frontend && npm run dev          # terminal 1
node docs/screenshots/take-screenshots.mjs
```

If you background the dev server, stop it with `pgrep`/`kill` — never
`pkill -f vite`, which also matches the shell running your own command (see
"LLM Operational Rules" in `AGENTS.md`). Cleanest is to spawn and kill it
inside a single script.

**Mocks must be grounded in the Go handlers, not guessed.** Read the handler
and its response struct before adding or editing a mock:

- Field names come from the `json:"..."` tags, and several types have **none** —
  `tailscale.StatusSummary` and `tailscale.PeerInfo` serialize as Go field
  names (`Connected`, `MagicDNSName`, `IPv4`), while `config.ServeRelay` is
  snake_case. `/api/auth/status` is camelCase (`needsSetup`). Don't assume.
- List endpoints wrap differently per type: `/api/serve/tcp/list` nests as
  `{relay, running}`, while the HTTPS and funnel lists **flatten** the embedded
  `ServeRelay` and add `hostname`/`running` (plus `listener_scheme` for HTTPS).
- `/api/logs` returns `{logs, level}`, not a bare array.

**A missed mock silently produces wrong screenshots.** An unmocked endpoint
falls through to a real backend; its 401 logs the app out mid-run, so the
"dashboard" captures come out as the login screen. The script guards this with
an `**/api/**` catch-all registered *first* (Playwright matches
most-recently-registered routes first, so specific handlers still win) that
warns and returns `{}`. When adding an endpoint to the frontend, add its mock
here too, and watch the run output for `unmocked API request`.

**Theme switching must go through the store.** Writing `localStorage` and
toggling the `dark` class repaints Tailwind colors but does not notify the
`theme` store — and the brand icon is a `derived` store off it
(`webui/frontend/src/lib/stores/theme.js`), so the logo keeps the previous
theme's PNG and washes out. `setTheme()` clicks the real navbar toggle
(`button[title="Toggle theme"]`, visible at all breakpoints) and falls back to
a reload only on views without one (login, setup).

**Always review the output images before committing.** Every failure mode above
produces a valid-looking PNG. Open the captures, don't just trust the exit code.

---

## 2. CHANGELOG.md

### Format

The project uses [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) format with [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

### Entry Structure

```markdown
## [X.Y.Z] - YYYY-MM-DD

Optional short description of the release theme.

### Added
- **Feature name** — description

### Changed
- Description of changed behaviour

### Fixed
- Description of bug fix

### Removed
- What was removed and why

### Security
- CVE or security fix description

### Upgrade Notes
- Any manual steps required when upgrading

### Docker
\`\`\`
docker pull sudocarlos/tailrelay:vX.Y.Z
\`\`\`
```

### Rules

- Newest version at the top, oldest at the bottom
- Use `### Added`, `### Changed`, `### Fixed`, `### Removed`, `### Security`, `### Upgrade Notes`, `### Docker` — only include sections that have content
- Sub-headings under `### Added` (e.g. `#### Frontend`, `#### Authentication`) are allowed for large releases with many additions
- Each bullet starts with `- **Bold label** — description` or just `- description` for minor items
- Date format: `YYYY-MM-DD`
- Docker section: always include the exact `docker pull` command with the new tag

### Adding a New Entry (checklist)

- [ ] Determine version bump: MAJOR (breaking), MINOR (new feature), PATCH (bug fix)
- [ ] Add `## [X.Y.Z] - YYYY-MM-DD` header above previous latest entry
- [ ] Fill in only sections that have content
- [ ] Include `### Upgrade Notes` if users need to take action
- [ ] Include `### Docker` with the pull command
- [ ] Update `TAILRELAY_VERSION` in `start.sh` to match the new version

---

## 3. webui/README.md

### Structure

1. Title + one-line description
2. Features list
3. Building section (`go build` command)
4. Running section (CLI flags)
5. Configuration section (key `webui.yaml` settings)
6. API endpoint listing

### What to Update

**New API endpoint added:**
- Add to the API endpoint listing with method, path, and description

**New config setting:**
- Add to the Key Settings list with name and description

**New CLI flag:**
- Add to the Running section

**New Web UI feature:**
- Add to the Features list

---

## 4. AGENTS.md

`AGENTS.md` is the entry point for all coding agents. It must stay accurate as the source of truth for agent onboarding.

### Sections to Maintain

**Skills Directory table** — update when a new skill is created or renamed:
```markdown
| Skill | Path | When to Use |
|-------|------|-------------|
| **My Skill** | `.agents/skills/my-skill/SKILL.md` | Description of when to load |
```

**File Map** — update when files are added/removed from the repo root or top-level directories:
```markdown
├── new-file.sh             # Brief description
```
Note: the correct path is `.agents/workflows/` (with an `s`).

**Environment Variables table** — update when env vars are added, removed, or have default changes:
```markdown
| `MY_VAR` | `default` | Purpose description |
```

**Quick Reference Commands** — update when Make targets, test commands, or health check endpoints change.

---

## 5. SKILL.md Files

Each skill file has a YAML front matter block:

```yaml
---
name: skill-name
description: One-sentence description used by the skill loader to decide when to activate this skill.
---
```

### When to Update a Skill

- A component's API, file structure, or behaviour changed
- New commands or Make targets were added relevant to the skill
- A Common Pitfall was discovered
- A covered path changed since the skill was last touched (check with `git log --oneline <prev-touch-commit>..HEAD -- <covered-paths>`, where `<prev-touch-commit>` is the commit that last edited the skill — not a tracked `reviewed_at` SHA)

### Skill File Locations

| Skill | File | Covers |
|-------|------|--------|
| `serve-relay-management` | `.agents/skills/serve/SKILL.md` | `webui/internal/serve/`, `webui/internal/handlers/serve.go` |
| `webui-development` | `.agents/skills/webui/SKILL.md` | `webui/`, `Makefile` |
| `docker-ci-pipeline` | `.agents/skills/docker-ci/SKILL.md` | `Dockerfile`, `.github/workflows/`, `compose-test.yml` |
| `tailscale-management` | `.agents/skills/tailscale/SKILL.md` | `webui/internal/tailscale/`, `start.sh` |
| `security-review` | `.agents/skills/security-review/SKILL.md` | All security-relevant code paths |
| `testing-cicd` | `.agents/skills/testing-cicd/SKILL.md` | `tests/`, `webui/internal/*/\*_test.go`, `.github/workflows/ci.yml` |
| `documentation` | `.agents/skills/documentation/SKILL.md` | All docs listed in this file |

---

## 6. Version Consistency Check

When releasing, these locations must all reflect the same version number:

| Location | How to Update |
|----------|--------------|
| `start.sh` line 3: `TAILRELAY_VERSION=vX.Y.Z` | Edit manually |
| `CHANGELOG.md` new entry header | Add new `## [X.Y.Z]` entry |
| GitHub Release tag | `git tag vX.Y.Z && git push --tags` |
| Docker Hub tag | Pushed automatically by CI on release |
| `AGENTS.md` → Version Information table in `docker-ci/SKILL.md` | Update if component versions changed |

```bash
# Quick check: version in start.sh
grep TAILRELAY_VERSION start.sh

# Quick check: latest CHANGELOG entry
head -10 CHANGELOG.md
```

---

## 7. Documentation Review Workflow

A full documentation review covers **every** document and **every** skill file. Run through all steps below in order.

### Step 1 — Find what changed since the docs were last touched

For each document and skill, find the commit that last edited it, then see
what changed in its covered paths since then:

```bash
# What changed in a covered path since a given commit
git log --oneline <commit>..HEAD -- <covered-paths>
```

Don't track a `reviewed_at` SHA per document — squash-merging makes those
short SHAs meaningless against `main`. When you need a baseline, use the
commit that last touched the doc/skill itself (`git log -1 --format=%H -- <path>`).

### Step 2 — Review and update all user-facing docs

| Document | What to check |
|----------|--------------|
| `README.md` | Feature list, Quick Start, version references, docs site link |
| `docs/openapi.yaml` | Every path/operation matches current handlers and routes |
| `website/` | Guide pages still accurate; `npm run build` succeeds |
| `CHANGELOG.md` | New entry for every release since last review |
| `webui/README.md` | API endpoint list, config settings, build commands |
| `AGENTS.md` | Skills table, File Map, env vars, Quick Reference commands |

For each stale document: read the diff (`git diff <commit>..HEAD -- <path>`)
and update the affected sections.

### Step 3 — Review and update ALL skill files

A full review must inspect every skill file, not just the ones whose covered paths changed. For each skill, read the file and verify its content reflects the current codebase.

Work through every skill in this order:

1. **`serve-relay-management`** — `.agents/skills/serve/SKILL.md`
   - Covers: `webui/internal/serve/`, `webui/internal/handlers/serve.go`
   - Check: relay types, ErrTailscaleNotReady, reconcile flow, API endpoints

2. **`webui-development`** — `.agents/skills/webui/SKILL.md`
   - Covers: `webui/`, `Makefile`
   - Check: build workflow, handler structure, auth flow, frontend SPA build

3. **`docker-ci-pipeline`** — `.agents/skills/docker-ci/SKILL.md`
   - Covers: `Dockerfile`, `.github/workflows/`, `compose-test.yml`
   - Check: pinned versions (Go, Node, Alpine, Tailscale), CI job names, Make targets

4. **`tailscale-management`** — `.agents/skills/tailscale/SKILL.md`
   - Covers: `webui/internal/tailscale/`, `start.sh`
   - Check: CLI wrapper methods, StatusCache, auth key handling, start.sh version

5. **`security-review`** — `.agents/skills/security-review/SKILL.md`
   - Covers: all security-relevant code paths
   - Check: known findings, checklist items, any new auth/input/backup code

6. **`testing-cicd`** — `.agents/skills/testing-cicd/SKILL.md`
   - Covers: `tests/`, `webui/internal/*/\*_test.go`, `.github/workflows/ci.yml`
   - Check: test package list, CI job names, integration test structure

7. **`documentation`** — `.agents/skills/documentation/SKILL.md` (this file)
   - Covers: all docs
   - Check: skill file table completeness, review workflow accuracy

For each skill: if its covered paths changed since it was last edited, read
the diff and update the affected sections. Commit conventions and PR/branch
guidance live in the global `conventional-commits` and `plan-workflow`
skills, not in a project-local skill.

### Step 4 — Commit

```bash
git add README.md CHANGELOG.md webui/README.md AGENTS.md .agents/skills/
git commit -m "docs: review and sync documentation with the codebase"
```
