# Git discipline

## Branch rules

- Always create a feature branch before making any changes
- Never commit directly to `main`, `master`, or `develop`
- Branch naming: `feat/PROJ-[id]-short-description` or `fix/PROJ-[id]-description`
- One branch per ticket — don't mix unrelated changes

```bash
git checkout -b feat/PROJ-42-short-description
```

## Commit message format

Conventional Commits — required:
```
type(scope): description [TICKET-ID]
```

Types: `feat` `fix` `docs` `test` `refactor` `chore` `perf`

Examples:
```
feat(auth): add token rotation [PROJ-42]
test(auth): add failing tests for token rotation [PROJ-42]
fix(api): refresh token not revoked on logout [PROJ-38]
```

**Separate commits for tests and implementation:**
```
test(scope): write failing tests for [feature] [TICKET-ID]
feat(scope): implement [feature] [TICKET-ID]
```

## Scope tags

Define scopes relevant to your project's domain boundaries. Example:

| Scope | When |
|---|---|
| `auth` | Authentication, sessions, tokens |
| `api` | Route handlers, request/response |
| `db` | Migrations, queries, schema |
| `infra` | Config, deployment, environment |
| `notify` | Email, SMS, push notifications |
| `admin` | Admin dashboard, roles |

## Before committing

Run `make done` and show output. Do not commit if it fails.

```bash
make done   # lint (go vet) + test + build + secrets (detect-secrets, skipped if not installed)
```

## PR rules

- Open a PR for every feature, no matter how small
- PR description must include:
  - What changed (brief)
  - Which ticket it resolves (`Closes PROJ-[id]`)
  - Output of `make done`
  - `git diff --stat` output listing all changed files
  - Any manual testing steps needed
- Tag auth/payment changes with `SECURITY: manual review required` in both code and PR description
- Add entry to `tech-debt.md` if any shortcuts were taken

## What Claude should never do

- `git push --force` to `main`, `master`, or `develop`
- Commit directly to a protected branch
- Mix multiple features in one commit or one branch
- Skip `make done` before committing