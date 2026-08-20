---
name: git-commit
description: >-
  Explicit-only workflow that drafts a Conventional Commits message and,
  unless --no-commit is supplied, commits the staged repository changes;
  supports --push, --auto-add, and --date.
---
# Git Commit

Draft a Conventional Commits message for this repository's changes and, by
default, run `git commit`. Pass `--no-commit` to only draft, `--push` to also
push, and `--date <date>` to set the commit date.

This skill analyzes the repository's changes and writes a commit message that
**MUST** follow the commit spec at `docs/development/commit-spec.md`
(Conventional Commits 1.0.0).

Run all commands from the repository root.

## When to Use This Skill

This skill is explicit-only. Use it only after the user invokes
`$git-commit` to draft, commit, or push repository changes.

## Invocation

```
$git-commit [--no-commit] [--push] [--auto-add] [--date <date>]
```

| Flag | Required | Values | Default |
| --- | --- | --- | --- |
| `--no-commit` | no | flag (no value) | off |
| `--push` | no | flag (no value) | off |
| `--auto-add` | no | flag (no value) | off |
| `--date <date>` | no | any git date string (see below) | — |

Behavior of the flags:

- **Default (no flags):** draft the message **and** commit **only what is already
  staged**. Do **not** run `git add` — the commit captures exactly the current
  staged index. If nothing is staged, report that and stop (see below).
- `--no-commit`: only draft and present the commit message. Do **not** stage,
  commit, or push anything.
- `--push`: run `git push` **after** the commit.
- `--auto-add`: let the agent decide which paths to stage before committing. With
  this flag the agent **may** run `git add <paths>` for the relevant changes
  (including untracked files) it judges belong in the commit, then commit them
  together with anything already staged.
- `--date <date>`: pass the value straight through to `git commit --date=<date>`.
  Accepts anything git accepts — a full timestamp (e.g. `2026-06-05T12:00:00`),
  an RFC/ISO date, or a relative expression (e.g. `"2 hours ago"`,
  `"yesterday"`).

Flag interactions and validation:

- `--push` and `--no-commit` are **mutually exclusive**. When `--no-commit` is
  set, `--push` is ignored (there is nothing new to push); the invocation behaves
  exactly like `--no-commit` alone.
- `--auto-add` only matters when a commit will happen. When `--no-commit` is set,
  `--auto-add` is ignored (nothing is staged or committed).
- Without `--auto-add`, the skill **never** runs `git add`; it commits the staged
  index as-is. If the staged index is empty, stop and report that there is
  nothing staged (point the user at `--auto-add` if they expected auto-staging).

## Commit Message Spec

The message **MUST** follow `docs/development/commit-spec.md` (Conventional
Commits 1.0.0). Read that file before drafting. Summary of the required shape:

```
<type>[optional scope][optional !]: <description>

[optional body]

[optional footer(s)]
```

- `type`: a noun such as `feat`, `fix`, `docs`, `refactor`, `chore`, `test`,
  `build`, `ci`, `perf`, `style`. `feat` for a new feature, `fix` for a bug fix.
- `scope` (optional): a noun in parentheses naming the affected area, e.g.
  `fix(scraper):`. Prefer a meaningful package or module scope.
- Description: short imperative summary right after `: `.
- Body (optional): one blank line after the description; free-form paragraphs
  explaining the *why*.
- Footers (optional): one blank line after the body; tokens use `-` for spaces
  (e.g. `Acked-by`), value after `: ` or ` #`.
- Breaking changes: a `!` before the `:` in the prefix, and/or a
  `BREAKING CHANGE:` footer (token MUST be uppercase).

### Examples

```
feat(catalog): support per-metric scale overrides via YAML

Let operators set a custom scale for any RDC field so memory metrics can
be reported in bytes instead of the default MB.
```

```
fix(scraper): skip GPUs that report no fields instead of erroring

A GPU with an empty field set no longer aborts the whole collection
cycle, so the remaining GPUs still expose their metrics.
```

## Steps

1. **Normalize flags.** If `--no-commit` is set, force draft-only mode and ignore
   both `--push` and `--auto-add`. Otherwise note whether `--auto-add` is set
   (controls whether the skill may `git add`) and whether `--push` is set. Capture
   the `--date` value, if any, to pass through to git verbatim.
2. **Read the spec.** Read `docs/development/commit-spec.md` and follow it.
3. **Inspect the changes:**

   ```bash
   git status
   git diff            # unstaged
   git diff --staged   # already staged
   git log --oneline -10   # match the repository's existing message style
   ```

   Base the drafted message on the content that will actually be committed: the
   **already-staged** changes by default, or the changes you intend to stage when
   `--auto-add` is set.
4. **Check there is something to commit** (skip when `--no-commit` is set):
   - **Without `--auto-add`:** if `git diff --staged` is empty, there is nothing
     staged — stop and report it (suggest staging manually or rerunning with
     `--auto-add`). Do not run `git add`.
   - **With `--auto-add`:** if the tree is entirely clean (no staged, unstaged, or
     untracked changes), stop and report that there is nothing to commit.
5. **Draft the message.** Summarize the nature and purpose of the changes into a
   Conventional Commits message. Focus the description/body on the *why*. Always
   present the drafted message to the user in a code block.
6. **Commit** (the default; **skip** when `--no-commit` is set). When
   `--date <date>` is provided, add `--date=<date>` and pass the value through
   unchanged. Use a HEREDOC so the formatting is preserved:

   - **Without `--auto-add` (default):** commit the staged index as-is — do
     **not** run `git add`.

     ```bash
     git commit [--date=<date>] -m "$(cat <<'EOF'
     <type>(<scope>): <description>

     <body>
     EOF
     )"
     ```

   - **With `--auto-add`:** stage the relevant paths you judged belong in the
     commit (including untracked files), then commit:

     ```bash
     git add <paths>            # stage the intended changes
     git commit [--date=<date>] -m "$(cat <<'EOF'
     <type>(<scope>): <description>

     <body>
     EOF
     )"
     ```

   When `--no-commit` is set, stop after presenting the message — do not stage,
   commit, or push.
7. **Push** (only when `--push` is set and `--no-commit` is **not** set): run
   after the commit.

   ```bash
   git push
   ```

8. **Verify**: `git status` is clean (when committed) and `git log -1` shows the
   new commit; report the commit hash and, if pushed, the push result.

## Git Safety

- Never change git config; never run destructive commands (`push --force`, hard
  reset) and never skip hooks (`--no-verify`) unless the user explicitly asks.
- Do not commit files that likely hold secrets (`.env`, credentials). Warn if the
  user explicitly requests it.
- Avoid `git commit --amend` unless the user asks and the standard amend
  preconditions hold.
- Never run `git add` unless `--auto-add` is set; by default commit only the
  already-staged index.
- If there is nothing to commit, report that and stop instead of creating an empty
  commit: by default that means an empty staged index; with `--auto-add` it means
  a fully clean tree (no staged, unstaged, or untracked changes).

## Notes

- Run all commands from the repository root.
- The skill writes one commit per invocation; rerun it to create additional
  commits.

