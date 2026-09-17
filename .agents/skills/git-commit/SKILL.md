---
name: git-commit
description: >-
  Explicit-only workflow that drafts a Conventional Commits message and,
  unless --no-commit is supplied, commits the staged repository changes;
  supports --push, --all, --auto-add, and --date, and adds the invoking
  agent's co-author trailer.
---
# Git Commit

Draft a Conventional Commits message for this repository's changes and, by
default, run `git commit`. Pass `--no-commit` to only draft, `--push` to also
push, `--all` to stage the complete working tree, `--auto-add` to let the agent
select paths, and `--date <date>` to set the commit date. With neither staging
flag, commit only the existing staged index.

This skill analyzes the repository's changes and writes a commit message that
**MUST** follow the commit spec at `docs/development/commit-spec.md`
(Conventional Commits 1.0.0). Every drafted or committed message **MUST** end
with the invoking agent's own canonical co-author identity in this form:

```text
Co-authored-by: AGENT_NAME <AGENT_EMAIL>
```

`AGENT_NAME` and `AGENT_EMAIL` above are placeholders and **MUST** be replaced.
Use the name and email declared for the current agent by its runtime, provider,
or system instructions. Do not assume that the current agent is Codex, copy an
identity from a previous commit, or use the repository's Git user/committer
identity. If no canonical agent email is available, stop and ask the user for
the co-author identity instead of inventing one.

Run all commands from the repository root.

## When to Use This Skill

This skill is explicit-only. Use it only after the user invokes
`$git-commit` to draft, commit, or push repository changes.

## Invocation

```
$git-commit [--no-commit] [--push] [--all | --auto-add] [--date <date>]
```

| Flag | Required | Values | Default |
| --- | --- | --- | --- |
| `--no-commit` | no | flag (no value) | off |
| `--push` | no | flag (no value) | off |
| `--all` | no | flag (no value) | off |
| `--auto-add` | no | flag (no value) | off |
| `--date <date>` | no | any git date string (see below) | — |

Behavior of the flags:

- **Default (no flags):** draft the message **and** commit **only what is already
  staged**. Do **not** run `git add` — the commit captures exactly the current
  staged index. If nothing is staged, report that and stop (see below).
- `--no-commit`: only draft and present the commit message. Do **not** stage,
  commit, or push anything.
- `--push`: run `git push` **after** the commit.
- `--all`: stage every tracked change, deletion, and non-ignored untracked file
  with exactly `git add --all`, then commit the resulting index. This mode does
  not let the agent select or omit paths.
- `--auto-add`: let the agent decide which paths to stage before committing. With
  this flag the agent **may** run `git add <paths>` for the relevant changes
  (including untracked files) it judges belong in the commit, then commit them
  together with anything already staged.
- `--date <date>`: set both the author date and committer date. Accept an
  absolute timestamp, an RFC/ISO date, or a relative expression such as
  `"2 hours ago"` or `"yesterday"`. Resolve the input once, immediately before
  committing, to an absolute ISO 8601 timestamp with a numeric UTC offset. Pass
  that resolved value to `git commit --date` and `GIT_COMMITTER_DATE`; never pass
  a relative expression directly to `GIT_COMMITTER_DATE`.

Flag interactions and validation:

- `--push` and `--no-commit` are **mutually exclusive**. When `--no-commit` is
  set, `--push` is ignored (there is nothing new to push); the invocation behaves
  exactly like `--no-commit` alone.
- `--all` and `--auto-add` are mutually exclusive staging policies. If both are
  supplied, stop and ask the user to choose one.
- `--all` and `--auto-add` only matter when a commit will happen. When
  `--no-commit` is set, both are ignored (nothing is staged or committed).
- Without `--all` or `--auto-add`, the skill **never** runs `git add`; it commits
  the staged index as-is. If the staged index is empty, stop and report that
  there is nothing staged (point the user at `--all` or `--auto-add` if they
  expected staging).

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
- Invoking-agent co-author trailer (required): resolve the current agent's own
  canonical name and email, then append `Co-authored-by: <name> <email>` as the
  final footer. Add it to draft-only output as well as messages passed to
  `git commit`. If that exact identity is already present, keep one copy and do
  not add a duplicate. Preserve any other valid trailers before it. Do not add
  another agent's identity unless the user explicitly requests it.

### Examples

```
feat(catalog): support per-metric scale overrides via YAML

Let operators set a custom scale for any RDC field so memory metrics can
be reported in bytes instead of the default MB.

Co-authored-by: AGENT_NAME <AGENT_EMAIL>
```

```
fix(scraper): skip GPUs that report no fields instead of erroring

A GPU with an empty field set no longer aborts the whole collection
cycle, so the remaining GPUs still expose their metrics.

Co-authored-by: AGENT_NAME <AGENT_EMAIL>
```

## Steps

1. **Normalize flags.** If `--no-commit` is set, force draft-only mode and ignore
   `--push`, `--all`, and `--auto-add`. Otherwise reject an invocation containing
   both `--all` and `--auto-add`; note the selected staging policy and whether
   `--push` is set. Capture the complete `--date` value, if any, for later
   resolution without changing its words or dropping its quoting boundary.
2. **Read the spec.** Read `docs/development/commit-spec.md` and follow it.
3. **Inspect the changes:**

   ```bash
   git status
   git diff            # unstaged
   git diff --staged   # already staged
   git log --oneline -10   # match the repository's existing message style
   ```

   Base the drafted message on the content that will actually be committed: the
   **already-staged** changes by default, every repository change with `--all`,
   or the paths you intend to stage with `--auto-add`.
4. **Check there is something to commit** (skip when `--no-commit` is set):
   - **Without a staging flag:** if `git diff --staged` is empty, there is
     nothing staged — stop and report it (suggest staging manually or rerunning
     with `--all` or `--auto-add`). Do not run `git add`.
   - **With `--all`:** if the tree is entirely clean, stop and report that there
     is nothing to commit. Otherwise stage every change with `git add --all`.
   - **With `--auto-add`:** if the tree is entirely clean (no staged, unstaged, or
     untracked changes), stop and report that there is nothing to commit.
5. **Draft the message.** Summarize the nature and purpose of the changes into a
   Conventional Commits message. Focus the description/body on the *why*. Append
   the invoking agent's resolved co-author trailer exactly once as the final
   footer. Never leave the `AGENT_NAME` or `AGENT_EMAIL` placeholders in an
   actual draft. Always present the complete drafted message, including the
   trailer, to the user in a code block.
6. **Commit** (the default; **skip** when `--no-commit` is set). When
   `--date <date>` is provided, resolve it once against the current clock and
   timezone before committing. On GNU systems, the equivalent operation is:

   ```bash
   RESOLVED_DATE="$(date --date="$REQUESTED_DATE" '+%Y-%m-%dT%H:%M:%S%:z')"
   ```

   Treat `REQUESTED_DATE` and `RESOLVED_DATE` as explanatory variable names, not
   fixed environment variables. If the date is invalid or the platform requires
   a different date parser, stop or use that platform's safe equivalent. With a
   resolved date, every commit command below must be invoked in this form so the
   author and committer dates represent the same instant:

   ```bash
   GIT_COMMITTER_DATE="$RESOLVED_DATE" \
     git commit --date="$RESOLVED_DATE" ...
   ```

   Without `--date`, omit both `GIT_COMMITTER_DATE` and `--date`. Use a HEREDOC
   so the message formatting is preserved:

   - **Without a staging flag (default):** commit the staged index as-is — do
     **not** run `git add`.

     ```bash
     git commit [resolved date environment/option, if requested] -m "$(cat <<'EOF'
     <type>(<scope>): <description>

     <body>

     Co-authored-by: AGENT_NAME <AGENT_EMAIL>
     EOF
     )"
     ```

   - **With `--auto-add`:** stage the relevant paths you judged belong in the
     commit (including untracked files), then commit:

     ```bash
     git add <paths>            # stage the intended changes
     git commit [resolved date environment/option, if requested] -m "$(cat <<'EOF'
     <type>(<scope>): <description>

     <body>

     Co-authored-by: AGENT_NAME <AGENT_EMAIL>
     EOF
     )"
     ```

   - **With `--all`:** stage the complete working tree without path selection,
     then commit:

     ```bash
     git add --all
     git commit [resolved date environment/option, if requested] -m "$(cat <<'EOF'
     <type>(<scope>): <description>

     <body>

     Co-authored-by: AGENT_NAME <AGENT_EMAIL>
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

8. **Verify**: `git status` is clean (when committed), `git log -1` shows the
   new commit, and the committed message contains the invoking agent's resolved
   co-author trailer exactly once. When `--date` was used, inspect
   `git show -s --format='%aI%n%cI' HEAD` and confirm both the author and committer
   timestamps equal the resolved instant. Report the commit hash, the resolved
   timestamp when applicable, and, if pushed, the push result.

## Git Safety

- Never change git config; never run destructive commands (`push --force`, hard
  reset) and never skip hooks (`--no-verify`) unless the user explicitly asks.
- Do not commit files that likely hold secrets (`.env`, credentials). If
  `--all` would include one, stop and warn instead of silently excluding it;
  `--all` must never be reinterpreted as agent-selected staging.
- Avoid `git commit --amend` unless the user asks and the standard amend
  preconditions hold.
- Never run `git add` unless `--all` or `--auto-add` is set. `--all` must use
  exactly `git add --all`; `--auto-add` must name the agent-selected paths. By
  default commit only the already-staged index.
- If there is nothing to commit, report that and stop instead of creating an empty
  commit: by default that means an empty staged index; with `--all` or
  `--auto-add` it means a fully clean tree (no staged, unstaged, or untracked
  changes).

## Notes

- Run all commands from the repository root.
- The skill writes one commit per invocation; rerun it to create additional
  commits.
