---
name: git-workflow
description: Git workflow guidance for commits, branches, and pull requests
license: Apache-2.0
metadata:
  version: "1.0.0"
  author: agno-team
  tags: ["git", "version-control", "workflow"]
---
# Git Workflow Skill

You are a Git workflow assistant. Help users with commits, branches, and pull requests following best practices.

## Commit Message Guidelines

For commit message generation and validation, use `get_skill_script("git-workflow", "commit_message.py")`.

### Best Practices & Rules
- **body format:** use list items instead of sentences to explain changes.
- **uniqueness:** each list item should describe something unique to avoid repetition.
- **casing:** all text in the commit message must be lowercase except when referencing code that uses different cases.
- **subject line:** use imperative mood (e.g., "add" instead of "added"), keep it concise (under 72 chars), and do not end with a period.
- **structure:** always separate the subject from the body with a blank line, and wrap body lines at ~72 characters.

### Format
```
<type>(<scope>): <subject>

- <unique change description>
- <another unique change description>

<footer>
```

### Types
- **feat**: new feature
- **fix**: bug fix
- **docs**: documentation only
- **style**: formatting, no code change
- **refactor**: code change that neither fixes a bug nor adds a feature
- **perf**: performance improvement
- **test**: adding or updating tests
- **chore**: maintenance tasks

### Examples
```
feat(auth): add oauth2 login support

- implement `OAuth2` authentication flow with google and github providers
- add token refresh mechanism and session management

closes #123
```

```
fix(api): handle null response from external service

- add null check before processing response data
- prevent `NullPointerException` when external service returns empty response

fixes #456
```

## Branch Naming

### Format
```
<type>/<ticket-id>-<short-description>
```

### Examples
- `feature/AUTH-123-oauth-login`
- `fix/BUG-456-null-pointer`
- `chore/TECH-789-update-deps`

## Pull Request Guidelines

### Title
Follow commit message format for the title.

### Description Template
```markdown
## Summary
Brief description of what this PR does.

## Changes
- Change 1
- Change 2

## Testing
How was this tested?

## Checklist
- [ ] Tests added/updated
- [ ] Documentation updated
- [ ] No breaking changes
```

## Common Commands

### Starting Work
```bash
git checkout main
git pull origin main
git checkout -b feature/TICKET-123-description
```

### Committing
```bash
git add -p  # Interactive staging
git commit -m "type(scope): description"
```

### Updating Branch
```bash
git fetch origin
git rebase origin/main
```

### Creating PR
```bash
git push -u origin feature/TICKET-123-description
# Then create PR on GitHub/GitLab
```
