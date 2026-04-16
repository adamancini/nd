# Contributing to nd

## Fork and Remote Layout

Personal development work uses a two-remote setup:

| Remote | Repo | Purpose |
|--------|------|---------|
| `origin` | `adamancini/nd` | Personal fork — push feature branches here |
| `upstream` | `paivot-ai/nd` | Canonical upstream — pull only |

```bash
git remote -v
# origin    git@github.com:adamancini/nd.git (fetch/push)
# upstream  git@github.com:paivot-ai/nd.git (fetch/push)
```

## Branch Model

All feature work uses git worktrees to allow parallel development:

```bash
# Create a worktree for a feature
git worktree add .worktrees/feat/my-feature -b feat/my-feature

# Work in the worktree
cd .worktrees/feat/my-feature

# Remove after merge
git worktree remove .worktrees/feat/my-feature
```

Worktrees live under `.worktrees/` (gitignored). Never create feature branches directly in the repo root.

## Syncing with Upstream

```bash
# Fetch latest upstream changes
git fetch upstream

# Rebase or merge main onto upstream
git checkout main
git merge upstream/main

# Push updated main to fork
git push origin main
```

## Contributing a Feature Upstream

For features worth contributing to `paivot-ai/nd`, create a clean `contrib/` branch from `upstream/main` and cherry-pick only the relevant commit(s):

```bash
# Fetch upstream
git fetch upstream

# Create contrib branch from upstream tip
git worktree add .worktrees/contrib/my-feature -b contrib/my-feature upstream/main

# Cherry-pick the feature commit (not local-only commits)
git -C .worktrees/contrib/my-feature cherry-pick <sha>

# Push to fork
git -C .worktrees/contrib/my-feature push origin contrib/my-feature

# Open PR from adamancini/nd:contrib/my-feature -> paivot-ai/nd:main
gh pr create --repo paivot-ai/nd --head adamancini:contrib/my-feature --base main
```

Keep `contrib/` branches lean: one logical feature per PR, no local-only commits (gitignore changes, `.claude/` config, etc.).

## Build and Test

```bash
make build      # produces ./nd binary
make install    # builds, installs to ~/go/bin, registers Claude Code plugin
make test       # go test -v ./...
make vet        # go vet ./...
```

Requires Go 1.23+.
