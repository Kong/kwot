# Conventional Commits Guide for kwot

This project uses **Conventional Commits** specification to automatically generate changelog and manage semantic versioning.

## Commit Message Format

```
<type>(<scope>): <subject>

<body>

<footer>
```

### Type
Must be one of the following:

- **feat**: A new feature
- **fix**: A bug fix
- **docs**: Documentation changes
- **style**: Code style changes (formatting, missing semicolons, etc.)
- **refactor**: Code refactoring without feature or bug fix
- **perf**: Performance improvements
- **test**: Adding or updating tests
- **chore**: Build process, dependency updates, tooling changes
- **ci**: CI/CD configuration changes

### Scope (Optional)
Specify what part of the codebase is affected:
- workspace
- roles
- validation
- config
- cli
- etc.

### Subject
- Use imperative mood ("add" not "added" or "adds")
- Don't capitalize first letter
- No period at the end
- Limit to 50 characters

### Body (Optional)
- Use imperative mood
- Explain what and why, not how
- Wrap at 72 characters
- Separate from subject with blank line

### Footer (Optional)
- Reference issues: `Fixes #123`, `Closes #456`
- Breaking changes: `BREAKING CHANGE: description`

## Examples

### Simple fix
```
fix: handle workspace creation race condition with linear backoff
```

### Feature with scope
```
feat(workspace): add availability check after creation

Add GET endpoint verification with linear backoff (50-250ms)
to handle Kong Enterprise replication lag when plugins fail
immediately after workspace creation.

Fixes #42
```

### Breaking change
```
feat!: redesign RBAC user structure

BREAKING CHANGE: RBAC user format changed from nested to flat structure.
Update config files accordingly.

Fixes #123
```

### With scope and breaking
```
feat(config)!: change yaml schema for workspace roles

BREAKING CHANGE: workspace.roles is now workspace.rbac_roles
```

## Automated Workflows

### Release Notes Generation
When you push a `v*` tag:
1. Release workflow reads commits since previous tag
2. Parses conventional commit prefixes (feat:, fix:, perf:)
3. Generates release notes with emoji indicators:
   - ✨ **Added** for feat commits
   - 🐛 **Fixed** for fix commits
   - ⚡ **Performance** for perf commits

### Changelog Update
1. Update Changelog workflow triggers on tag push
2. Extracts all commits since last tag
3. Groups by type (Added, Fixed, Performance)
4. Updates CHANGELOG.md automatically
5. Commits and pushes changes to main

## Best Practices

### ✅ Do

- Use lowercase types and scopes
- Be specific about what changed
- Reference related issues
- Use imperative mood ("add" not "added")
- Keep subject under 50 characters
- Provide context in body for complex changes

### ❌ Don't

- Mix types (feat AND fix in one commit)
- Make vague commits ("update stuff")
- Use capital letters in types
- Add period at end of subject
- Put detailed explanation in subject

## Examples in kwot

### Good commits
```
feat(workspace): add max retry attempts environment variable
fix(roles): resolve race condition with permission assignment
perf: improve workspace availability check backoff timing
docs: add MAX_RETRY_ATTEMPTS to README
chore: update dependencies
```

### Bad commits
```
updated stuff
Fixed some bugs
Added feature
WIP - don't merge
random changes
```

## Checking Before Commit

```bash
# View commit message format
git log --oneline -5

# Check commit message follows convention
git log -1 --format=%s

# See all commits between tags
git log v1.0.1..v1.0.2 --oneline
```

## Semantic Versioning

Based on conventional commits:

- **Major (x.0.0)**: Breaking changes (feat!, BREAKING CHANGE)
- **Minor (0.x.0)**: New features (feat:)
- **Patch (0.0.x)**: Bug fixes (fix:) and other changes

Current version: Check `Makefile` for VERSION

## Amending Commits

If you made a commit with wrong format:

```bash
# Amend the last commit
git commit --amend -m "feat(scope): corrected message"

# Force push (only on your branch!)
git push origin branch-name --force-with-lease
```

## Related Files

- `.github/workflows/release.yml` - Generates release notes from commits
- `.github/workflows/update-changelog.yml` - Updates CHANGELOG.md automatically
- `cliff.toml` - Configuration for git-cliff (optional, for advanced use)
