---
name: documentation
description: Updating all tailrelay documentation — README, CHANGELOG, release notes, AGENTS.md, SKILL.md files, webui/README.md, and the Docusaurus docs site. Use when adding user-facing features, releasing a new version, updating component versions, or when any doc's reviewed_at SHA is out of date with HEAD.
reviewed_at: 6f8a583
---

# Documentation

## Overview

tailrelay documentation spans six locations that must stay consistent with each other and with the source code:

| Document | Audience | Update Trigger |
|----------|----------|---------------|
| `README.md` | End users | Features, Quick Start, version, links to docs site |
| `docs/openapi.yaml` | API consumers | Any handler/route change in `webui/internal/handlers/`, `webui/internal/web/server.go` |
| `website/` | End users & developers | Guides (getting started, auth, dev, troubleshooting); rendered API reference is generated from `docs/openapi.yaml` automatically |
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

**Ownership split for guide content (Quick Start, Tailscale Setup,
Troubleshooting, etc.):** `website/docs/*.md` is the canonical, detailed
version. `README.md` keeps a condensed copy for GitHub browsing (Docker Hub,
`git clone`, no JS) — when one changes, check whether the other needs the
same fix. Prefer editing `website/docs/*.md` first, then trim the README's
copy to match rather than letting them diverge.

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

**Documentation Review Status table** — must be updated after any full document review:
```markdown
| Document | `reviewed_at` | Paths Covered |
|----------|---------------|---------------|
| `AGENTS.md` | `<new-sha>` | `AGENTS.md`, `Makefile`, `start.sh` |
```

How to find what changed since last review:
```bash
git log --oneline <reviewed_at>..HEAD -- <covered-paths>
```

Update `reviewed_at` to current HEAD SHA after completing a review:
```bash
git rev-parse --short HEAD
```

---

## 5. SKILL.md Files

Each skill file has a YAML front matter block:

```yaml
---
name: skill-name
description: One-sentence description used by the skill loader to decide when to activate this skill.
reviewed_at: <git-sha>
---
```

### When to Update a Skill

- A component's API, file structure, or behaviour changed
- New commands or Make targets were added relevant to the skill
- A Common Pitfall was discovered
- The skill's `reviewed_at` SHA is behind HEAD for covered paths

### Checking if a Skill Needs Updating

```bash
# Check which covered files changed since the skill was last reviewed
git log --oneline <reviewed_at>..HEAD -- <covered-paths>

# Example for webui skill (reviewed_at=17791f3, covers webui/)
git log --oneline 17791f3..HEAD -- webui/ Makefile
```

### Skill File Locations

| Skill | File | Covers |
|-------|------|--------|
| `serve-relay-management` | `.agents/skills/serve/SKILL.md` | `webui/internal/serve/`, `webui/internal/handlers/serve.go` |
| `webui-development` | `.agents/skills/webui/SKILL.md` | `webui/`, `Makefile` |
| `docker-ci-pipeline` | `.agents/skills/docker-ci/SKILL.md` | `Dockerfile`, `.github/workflows/`, `compose-test.yml` |
| `tailscale-management` | `.agents/skills/tailscale/SKILL.md` | `webui/internal/tailscale/`, `start.sh` |
| `git-workflow` | `.agents/skills/git-workflow/SKILL.md` | Commit conventions, branch naming |
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

### Step 1 — Get current HEAD SHA

```bash
git rev-parse --short HEAD   # this becomes the new reviewed_at for everything you review
```

### Step 2 — Check staleness for every tracked document

For each row in the AGENTS.md Documentation Review Status table, run:

```bash
git log --oneline <reviewed_at>..HEAD -- <covered-paths>
```

If the output is non-empty the document is stale and must be updated.

### Step 3 — Review and update all user-facing docs

| Document | What to check |
|----------|--------------|
| `README.md` | Feature list, Quick Start, version references, docs site link |
| `docs/openapi.yaml` | Every path/operation matches current handlers and routes |
| `website/` | Guide pages still accurate; `npm run build` succeeds |
| `CHANGELOG.md` | New entry for every release since last review |
| `webui/README.md` | API endpoint list, config settings, build commands |
| `AGENTS.md` | Skills table, File Map, env vars, Quick Reference commands |

For each stale document: read the diff (`git diff <reviewed_at>..HEAD -- <path>`), update the affected sections, then mark it reviewed.

### Step 4 — Review and update ALL skill files

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

7. **`git-workflow`** — `.agents/skills/git-workflow/SKILL.md`
   - Covers: commit conventions, branch naming
   - Check: commit types, branch format, PR template

8. **`documentation`** — `.agents/skills/documentation/SKILL.md` (this file)
   - Covers: all docs
   - Check: skill file table completeness, review workflow accuracy

For each skill:
- If covered paths changed: read the diff and update affected sections
- In all cases: update `reviewed_at` in the front matter to the current HEAD SHA

### Step 5 — Update AGENTS.md Documentation Review Status table

Advance every `reviewed_at` cell you touched to the current HEAD SHA.

### Step 6 — Commit

```bash
git add README.md CHANGELOG.md webui/README.md AGENTS.md .agents/skills/
git commit -m "docs: update documentation for review at <new-sha>"
```
