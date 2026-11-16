**For: Engineering Team**

**Maintained by: Team Lead / Senior Engineer**

**Version: 1.0**

---

## Purpose

This playbook defines how the engineering team uses Claude Code to build production-grade applications. Every engineer working with Claude Code on this team follows this process — it is not optional guidance, it is the workflow.

The core principle: **Claude Code is powerful but undisciplined by default. This playbook is the discipline.**

---

## Table of Contents

1. Phase 1 — Requirements & Definition of Done
2. Phase 2 — Project Setup & Guardrails
3. Phase 3 — Plan Mode (Mandatory)
4. Phase 4 — Implementation Rules
5. Phase 5 — Quality Gates
6. Phase 6 — Multi-Session Management
7. Reference: File Structure & Git Hygiene
8. Reference: CLAUDE.md Template
9. Reference: .claude/rules/ Templates
10. Reference: PRD Template
11. Reference: tech-debt.md
12. Reference: Token Optimization Tools
13. Quick Reference Card

---

<a name="phase-1"></a>

## Phase 1 — Requirements & Executable Definition of Done

### 1.1 Write a PRD before opening Claude Code

Every feature must have a PRD written before any session begins. Store it at `docs/PRD-[feature-name].md`. Required fields:

| Field | Description |
| --- | --- |
| Problem statement | What pain does this solve, and for whom? |
| Scope boundary | What is explicitly OUT of scope? |
| Key user flows | 2–3 core journeys |
| Tech constraints | Language, framework, existing integrations |
| Acceptance criteria | Testable `Given / When / Then` conditions |
| Implementation Status | Tracking table — updated as phases complete |
| Key decisions | Open questions that must be answered before plan mode |
| Definition of Done | `make done` |

See the full fillable template in Reference: PRD Template.

---

### 1.2 Write acceptance criteria before writing code

Acceptance criteria must exist **before** Claude Code writes a single line of production code. Format:

```
Given [context / precondition]
When  [action]
Then  [expected outcome]
```

These become the test stubs in Phase 4 (TDD). Claude writes tests to match each criterion, confirms they fail, then implements until they pass.

---

### 1.3 Create an executable Definition of Done

"Done" is a command that returns pass or fail — not a judgment call. Create a `done` target in the project `Makefile`:

```makefile
lint:
	go vet -tags=integration ./...

test:
	go test ./... -race -cover

build:
	go build -o bin/app .

# ponytail: skips with a notice when detect-secrets is not installed.
secrets:
	@if command -v detect-secrets >/dev/null 2>&1; then \
		git ls-files -z | xargs -0 detect-secrets-hook --; \
	else echo "secrets: detect-secrets not installed, skipped"; fi

done: lint test build secrets
	@echo "make done: OK"
```

End every implementation session with:

> `"Run make done. Show the full output. If it fails, fix every failure before stopping."`
> 

This transforms "looks done" into "provably done."

---

### 1.4 System design context — how to give it to Claude

**Do not paste PRDs or architecture docs into chat.** This burns context window and does not persist across sessions.

**Keep all docs in `docs/` at the repo root** — not inside `.claude/`. These documents are used by the whole team, not just Claude.

```
docs/
├── ARCHITECTURE.md     # System overview, key decisions, data flow (≤ 300 lines)
├── API-contracts.md    # Endpoint specs, request/response shapes
├── DATA-MODELS.md      # Entity definitions and relationships
├── PRD-[feature].md    # Per-feature product briefs
└── DECISIONS.md        # Architectural decision log (ADR format)
```

Reference them in `CLAUDE.md` so Claude loads them via `@` references — not by copy-pasting into chat:

```
## System context
- Architecture: @docs/ARCHITECTURE.md
- API contracts: @docs/API-contracts.md
- Data models:  @docs/DATA-MODELS.md
```

At session start for a specific feature, reference the PRD directly in the prompt:

```
Read @docs/ARCHITECTURE.md and @docs/PRD-[feature].md.
Read @internal/[module]/handler/[file].go to understand existing patterns.
Enter plan mode. Do not code yet.
```

Keep `ARCHITECTURE.md` under 300 lines — it loads every session. Detail lives in the other docs files.

---

### 1.5 PRD Implementation Status tracking

Every PRD includes an **Implementation Status table** directly below the header. Claude checks this at session start and updates it as phases complete. Engineers update the PR column on merge.

```markdown
## Implementation Status
<!-- Claude: check this table at session start.
     Verify each component by checking whether the key file exists.
     Update status and PR as each phase completes. -->

| Component        | Status   | PR | make done |
|---|---|---|---|
| Domain structs   | ⏳ Todo  | —  | —         |
| DB migration     | ⏳ Todo  | —  | —         |
| Repository layer | ⏳ Todo  | —  | —         |
| Service layer    | ⏳ Todo  | —  | —         |
| Handler layer    | ⏳ Todo  | —  | —         |
| Tests ≥ 80%      | ⏳ Todo  | —  | —         |

Status key: ⏳ Todo · 🔄 In Progress · ✅ Done · 🚫 Blocked
```

Claude verifies status by checking whether the key files exist (`ls internal/booking/handler/booking.go`) rather than trusting the table alone.

---

<a name="phase-2"></a>

## Phase 2 — Project Setup & Guardrails

Run this setup **once per project**. Every subsequent session inherits these constraints automatically.

### 2.1 Required file structure

```
project-root/
├── CLAUDE.md                    # Project brain — Claude reads every session
├── PLAN.md                      # Active execution plan (committed to repo)
├── tech-debt.md                 # Deferred work registry (committed to repo)
├── Makefile                     # Includes `make done`
├── .env.example                 # Schema only — no real values ever committed
├── .gitignore
├── docs/
│   ├── ARCHITECTURE.md
│   ├── API-contracts.md
│   ├── DATA-MODELS.md
│   └── PRD-[feature].md
├── .claude/
│   ├── rules/
│   │   ├── testing.md           # TDD rules + exploration policy
│   │   ├── security.md          # Never-do list + manual review triggers
│   │   ├── api-patterns.md      # Endpoint conventions
│   │   └── git.md               # Branch and commit rules
│   └── context/                 # Optional: condensed token-saving summaries
```

### 2.2 Git discipline

- Always create a feature branch before making any changes — **never commit to `main` or `develop`**
- Branch naming: `feat/PROJ-[id]-short-description` or `fix/PROJ-[id]-description`
- Commit format (conventional commits): `type(scope): description [PROJ-ID]`
- Separate commits for tests and implementation
- Open a PR for every feature, no matter how small

Full rules live in `.claude/rules/git.md` — see Reference: .claude/rules/.

### 2.3 Token optimization tools

Install these once. They reduce cost and output noise across every session.

**caveman** — cuts ~65–75% of output tokens by compressing Claude's prose while keeping every technical detail, code block, and error string intact:

```bash
curl -fsSL https://raw.githubusercontent.com/JuliusBrussee/caveman/main/install.sh | bash
```

Mid-session commands: `/caveman lite` · `/caveman` (default) · `/caveman ultra` · `"normal mode"` to disable.

**ponytail** — enforces "laziest senior dev" coding: stdlib first, no unasked-for abstractions, deletion over addition:

```bash
# Inside a Claude Code session:
/plugin marketplace add DietrichGebert/ponytail
/plugin install ponytail@ponytail
# /hooks → trust the two lifecycle hooks → start a new thread
```

Key commands: `/ponytail-review` (scans current diff for over-engineering, returns a delete-list) · `/ponytail ultra` (questions whether the task needs to exist at all).

> These are Claude Code plugins — they operate at the AI layer, not the code layer. They are not Go libraries.
> 

---

<a name="phase-3"></a>

## Phase 3 — Plan Mode (Mandatory for All Implementation Tasks)

**Claude must never write production code without a reviewed plan. No exceptions.**

### 3.1 Where plan mode is specified

The "always enter plan mode" rule lives in **CLAUDE.md** — see Reference: CLAUDE.md Template. It is a global behavioral default, not a per-session instruction. `.claude/rules/` files are for topic-specific rules (TDD, security, API patterns) referenced contextually.

### 3.2 The four-step planning workflow

**Step 1 — Enter plan mode and explore**

```
Enter plan mode.
Read @docs/ARCHITECTURE.md and @docs/PRD-[feature].md.
Check the Implementation Status table in the PRD.
List every file that will be created or modified.
Do not make any changes yet.
```

**Step 2 — Draft plan.md**

```
Create plan.md with:
- Implementation phases (numbered)
- Files to create or modify under each phase
- Files NOT to touch (scope boundary)
- Key decisions with rationale
- Risks or open questions
- Test scenarios to cover
Do not implement yet.
```

**Step 3 — Engineer annotates plan.md**

Open `plan.md` in your editor. Add `> NOTE:` comments inline at every point where Claude made the wrong call, was ambiguous, or missed a constraint:

```markdown
## Phase 2: Repository layer
Add HasOverlap query for conflicting bookings.
> NOTE: exclude status 'cancelled' AND 'rejected' — not just 'cancelled'
> NOTE: use COUNT(*) OVER() window function for list queries, not a separate count

## Phase 3: Service layer
> NOTE: compare date only, not datetime — leave is always full-day
> NOTE: return ErrNotFound on cross-member access, not ErrForbidden
```

Send the annotated file back:

```
Address all notes in plan.md. Do not implement yet. Update plan.md with the revised approach.
```

Repeat until zero unresolved notes.

**Step 4 — Implement phase by phase**

```
Exit plan mode.
Implement Phase 1 from plan.md.
Only touch files listed under Phase 1. Do not modify [out-of-scope paths].
Run make done. Show the full output. Fix any failures before stopping.
```

After each phase, update the Implementation Status table in the PRD.

### 3.3 When plan mode is NOT required

Skip the plan only when the entire change can be described in one sentence: a typo fix, a single config value, renaming a variable.

---

<a name="phase-4"></a>

## Phase 4 — Implementation Rules

### 4.1 Prompting discipline

| Rule | Do this | Not this |
| --- | --- | --- |
| Reference files | `"Follow the pattern in @internal/booking/handler/booking.go"` | `"Follow the booking handler pattern"` |
| Point to existing patterns | `"Implement the interface from @internal/repository/booking.go"` | `"Add a repository"` |
| Scope explicitly | `"Only touch internal/booking/. Do not modify internal/auth/."` | `"Update the booking system"` |
| Fix root causes | `"Fix the root cause. Do not suppress errors."` | `"Make the test pass"` |
| Verify outcomes | `"Run make done. Show the output. Fix failures."` | `"Looks good, commit it"` |

If a prompt has no file scope, no acceptance criteria, and no ticket reference — add them before sending. Claude is instructed in CLAUDE.md to ask for this context before implementing.

### 4.2 TDD workflow

Full rules live in `.claude/rules/testing.md` — see Reference: .claude/rules/. The mandatory sequence:

1. Write tests for every acceptance criteria — **before** any production code
2. Run tests. Confirm they **fail**. Show the failure output
3. Implement until all tests pass
4. Commit tests and implementation as **separate commits**

Beyond the given ACs, Claude must explore and add tests for:

- Null / empty / zero inputs
- Boundary values (min, max, off-by-one)
- Error and failure paths (DB down, external service timeout)
- Idempotency (same webhook or request received twice)
- Concurrency (`race` flag — two users hitting the same resource simultaneously)
- Permission edges (cross-member access, unauthenticated, wrong role)
- State transitions — valid and invalid (e.g. cancelling an approved request)

Claude lists the additional scenarios it found with rationale, and flags any that need clarification before implementing.

### 4.3 Context window management

Context is the scarcest resource. When it fills, Claude starts forgetting earlier instructions and contradicting constraints.

- Use `/compact` mid-session when context is filling but you are still on the same feature
- Start a **fresh session** for each new feature phase — carry state via PLAN.md
- Keep `CLAUDE.md` under 200 lines — it loads fully into every session
- Load `.claude/rules/` files contextually, not all at once

**Signal that context is degrading:** Claude contradicts earlier instructions, reverts to patterns you already corrected, or ignores constraints it had been following. `/compact` immediately and re-anchor with the session handoff prompt (see Phase 6.3).

### 4.4 Scope control

Always state explicitly what Claude must not touch:

```
Only modify files in internal/payments/.
Do not touch the handler layer, auth middleware, or any test fixtures outside payments/.
```

Without this, Claude makes "helpful" changes outside scope that silently break unrelated things.

---

<a name="phase-5"></a>

## Phase 5 — Quality Gates

### 5.1 The enforcement model

| Layer | Tool | When it runs | Bypassable? |
| --- | --- | --- | --- |
| AI behavior | CLAUDE.md rules | Every session | Yes — Claude can drift |
| Automated gate | `make done` | Before every commit | Yes — nothing runs it automatically, so never commit without it |

Both layers must be in place. `make done` is the only automated gate; it backs up the CLAUDE.md rules.

### 5.2 Pre-merge checklist

Run this before requesting any PR review:

```
[ ] make done passes — `lint` (go vet), `test`, `build` and `secrets` (detect-secrets; skipped with a notice if not installed)
[ ] PRD Implementation Status table updated with PR number
[ ] No secrets or credentials in the diff
[ ] Tests cover all acceptance criteria and edge cases from .claude/rules/testing.md
[ ] Error handling present on all external calls
[ ] Auth / permission changes tagged // SECURITY: manual review required
[ ] tech-debt.md updated if any shortcuts were taken
[ ] PR description: what changed, why, and which ticket it closes
```

### 5.3 Items requiring mandatory manual human review

Claude proposes these changes — a human engineer must review before merge:

- JWT validation and session management
- Authorization / permission checks
- Any new external HTTP call
- User input that reaches a DB query
- New dependencies (`go.mod`, `package.json` changes)
- Environment variable or secret handling
- Payment flow changes (webhook handling, amount calculation)

Claude marks these with `// SECURITY: manual review required` in code and flags them in the PR description.

### 5.4 AI-assisted code review pass

After implementation, open a **fresh session** (no anchoring bias from the implementing session) and prompt:

```
Read these files: @[list of changed files]

Review for:
1. Correctness — does it match the spec in @docs/PRD-[feature].md?
2. Edge cases — what scenarios are not covered by the tests?
3. Security — auth, input validation, injection risk?
4. Performance — N+1 queries, unbounded loops, missing indexes?
5. Observability — are errors logged? are key paths instrumented?

Rate each finding: Critical / High / Medium / Low.
```

A fresh session has no investment in the implementation decisions — it finds what the implementing session normalized.

---

<a name="phase-6"></a>

## Phase 6 — Multi-Session Management

### 6.1 The context window is not memory

Claude has no memory between sessions. State must live in files that Claude reads at the start of each new session.

| File | Owner | Purpose | In repo? |
| --- | --- | --- | --- |
| `CLAUDE.md` | Team Lead | Permanent project rules | ✅ Yes |
| `PLAN.md` | AI + Engineer | Active execution plan | ✅ Yes |
| `tech-debt.md` | AI + Engineer | Deferred work registry | ✅ Yes |
| `.claude/rules/*.md` | Team Lead | Topic-specific rules | ✅ Yes |

PLAN.md is committed because it is shared state — any engineer or new Claude session needs to know what phase the project is on, what decisions were made, and what is still open.

### 6.2 PLAN.md — execution state across sessions

Create PLAN.md at the end of Plan Mode. Keep it current as phases complete.

```markdown
# Plan: [Feature Name]
**Ticket:** PROJ-[id]
**Status:** 🔄 In Progress — Phase 2 of 4
**Last updated:** [date]

## Phases
- [x] Phase 1: Domain structs + DB migration — DONE (PR #12)
- [ ] Phase 2: Repository layer — IN PROGRESS
- [ ] Phase 3: Service layer — TODO
- [ ] Phase 4: Handler + routes — TODO

## Phase 2 detail
- [x] Create interface
- [ ] Implement HasOverlap query
- [ ] Implement ListByMember with pagination

## Decisions made — do not revisit
- PATCH not PUT for status updates
- ErrNotFound on cross-member access (not ErrForbidden)
- Slot hold TTL: 300s via SLOT_HOLD_TTL env var

## Open questions / blockers
- Cancellation window hours → PO: ⏳ pending
```

### 6.3 Session handoff prompt

Send this at the start of every new session to orient Claude:

```
Read CLAUDE.md, PLAN.md, and tech-debt.md.

We are on Phase [N]. Last session completed [what was done].
Continue with [next task].
Only touch files in [scoped path].
Run make done when complete. Show the full output.
```

### 6.4 When to compact vs when to start a new session

| Situation | Action |
| --- | --- |
| Context filling but still on the same feature | `/compact` — summarises context and continues |
| Moving to a new feature phase | New session — send the handoff prompt |
| Starting a different feature entirely | New session — update PLAN.md first, then handoff prompt |
| Claude contradicts earlier instructions | `/compact` immediately, re-anchor with handoff prompt |

---

<a name="ref-structure"></a>

## Reference: File Structure & Git Hygiene

### Commit to repo

```
CLAUDE.md               tech-debt.md            .claude/rules/*.md
PLAN.md                 docs/*.md               Makefile
.env.example
```

### Never commit

```
.env  .env.*            # real secrets — only .env.example goes in
bin/  dist/  .next/     # build artifacts
node_modules/ vendor/   # dependencies
*.out  coverage.html    # test output
```

### .gitignore minimum

```
.env
.env.*
!.env.example
bin/
dist/
.next/
out/
node_modules/
*.out
coverage.out
coverage.html
.DS_Store
```

---

<a name="ref-claudemd"></a>

## Reference: CLAUDE.md Template

```markdown
# Project: [Name]

## Purpose
[1–2 sentences: what this does and what problem it solves]

## Tech stack
- Backend: Go 1.22 (Gin)
- Frontend: React / Next.js 14
- Database: PostgreSQL 16 (pgx driver — never raw database/sql)
- Cache: Redis 7
- Payments: [e.g. Xendit + QRIS]
- Auth: JWT RS256 + Redis-stored refresh tokens

## System context — read before structural decisions
- Architecture:   @docs/ARCHITECTURE.md
- API contracts:  @docs/API-contracts.md
- Data models:    @docs/DATA-MODELS.md
- Feature PRDs:   @docs/PRD-[feature-name].md

## Key files — entry points for understanding patterns
- Handler pattern:    @internal/[module]/handler/[file].go
- Service pattern:    @internal/[module]/service/[file].go
- Repository pattern: @internal/[module]/repository/[file].go
- Auth middleware:    @internal/middleware/auth.go
- Response helpers:   @internal/response/response.go
- Input validation:   @internal/validator/validator.go
- DB pool:            @internal/db/db.go

## Commands
- Run:   `make run`
- Test:  `make test`
- Lint:  `make lint`
- Build: `make build`
- **Definition of Done: `make done`** — run before declaring any task complete. Show the full output.

---

## Behavior — always follow

- For ANY implementation task, start in plan mode (Shift+Tab × 2 or `/plan`). Create `plan.md` before writing any production code.
- Do not write production code until `plan.md` has been annotated and confirmed by the engineer. After receiving annotations, use the phrase "address all notes, don't implement yet."
- Always create a feature branch before making changes. Never commit to main.
- Run `make done` before declaring any task complete. Show the full output. If it fails, fix every failure before stopping.
- Do not touch files outside the stated scope of the current task.
- If a prompt has no file scope, no acceptance criteria, and no ticket reference — ask for them before implementing.

## TDD — no exceptions
1. Write tests BEFORE production code.
2. Run tests. Confirm they FAIL. Show the failure output before continuing.
3. Implement until all tests pass.
4. Commit tests and implementation as separate commits.
Full rules: @.claude/rules/testing.md

## Tech debt tracking
When making an intentional shortcut or deferring an improvement, add to `tech-debt.md` immediately:
```

- [ ]  TD-[N]: [What the shortcut is]
    - Impact: [Why it matters]
    - Source: [Current ticket/branch and phase]
    - Created: [date]

```

## PRD status
After each implementation phase completes, update the Implementation Status table in `@docs/PRD-[feature].md`.

---

## Hard rules — never break

**Secrets**
- Never commit secrets, API keys, tokens, or credentials — use env vars only
- If a secret is ever committed: rotate it immediately and treat it as compromised

**Code**
- Never `_ = err` — handle or propagate with context
- Never raw string interpolation in SQL — parameterized queries only
- Never trust client-reported amounts — calculate server-side
- Never expose stack traces in API responses
- Tag auth and payment changes: `// SECURITY: manual review required`

Full security rules: @.claude/rules/security.md
Full git rules:      @.claude/rules/git.md
Full API patterns:   @.claude/rules/api-patterns.md
```

---

<a name="ref-rules"></a>

## Reference: .claude/rules/ Templates

### testing.md

```markdown
# Testing rules

## Mandatory TDD workflow
1. Write tests BEFORE production code — always.
2. Run tests. Confirm they FAIL. Show failure output before continuing.
3. Implement until all tests pass.
4. Commit tests and implementation as separate commits:
   test(scope): add failing tests for [scenario] [PROJ-ID]
   feat(scope): implement [feature] [PROJ-ID]

## Beyond acceptance criteria
After covering every given AC, explore and add tests for:
- Null / empty / zero inputs — missing fields, empty strings, zero amounts
- Boundary values — min, max, off-by-one, empty collections
- Error paths — DB down, Redis timeout, external service unreachable
- Idempotency — same webhook or request received twice
- Concurrency — simultaneous requests on the same resource (run with -race)
- Permission edges — cross-member access, unauthenticated, wrong role
- State transitions — valid and invalid (e.g. cancelling an approved record → 409)

List the additional scenarios found with rationale. Flag any needing clarification before implementing.

## Test naming
TestFunction_Scenario_ExpectedOutcome
Example: TestCreateBooking_DuplicateWebhook_NoDoubleBooking

## Layer rules
Repository: use real test DB via testutil/db.go — never mock the DB layer.
Service:    mock the repository interface — no DB hits.
Handler:    httptest.NewRecorder() — mock the service — test every error → HTTP status mapping.

## Coverage target
≥ 80% on all internal packages. Run `go test ./... -cover` and include output in make done.
```

### security.md

```markdown
# Security rules

## Never do
- Commit secrets, API keys, tokens, or credentials
- Log sensitive fields: passwords, tokens, payment data, PII
- Disable TLS verification (InsecureSkipVerify)
- Use `_ = err` to silence errors
- Trust client-reported payment amounts or status
- Use raw string interpolation in SQL — parameterized queries only
- Store money as float — use INTEGER (smallest currency unit)
- Expose stack traces or internal error details in API responses

## Always do
- Validate all inputs at the handler boundary before passing to service layer
- Wrap all external HTTP calls with a context deadline (timeout)
- Verify payment webhook HMAC signatures before processing
- Return ErrNotFound (not ErrForbidden) on cross-member resource access
  — do not leak that the record exists

## Flag for manual review
Add `// SECURITY: manual review required` near the change and flag in the PR:
- JWT validation logic
- Payment webhook signature verification
- Any new external HTTP call
- Permission or role checks
- User input that reaches a DB query
- New dependencies (go.mod, package.json)
- Payment flow changes
```

### api-patterns.md

```markdown
# API patterns

## URL conventions
- Versioned: /api/v1/[resource]
- Plural nouns: /courts, /bookings, /members
- Nested max one level: /api/v1/courts/:id/availability
- Webhooks: /webhooks/[provider]
- Admin-only: /api/v1/admin/[resource]

## HTTP method semantics
GET — read only, no side effects
POST — create resource or trigger action (returns 201 + created resource)
PATCH — partial update (only provided fields change)
PUT — full replacement (all fields required)
DELETE — soft-delete (returns 204)

## Status codes
200 success (GET, PATCH, PUT)     201 created (POST)        204 no body (DELETE)
400 malformed request             401 unauthenticated        403 unauthorized
404 not found                     409 conflict               422 validation error
500 internal — log detail; never expose to client

## Error response shape
{ "error": { "code": "MACHINE_READABLE_CODE", "message": "...", "fields": {} } }

Validation errors include per-field detail in "fields".

## Success response shape
Single:     { "data": { ...resource... } }
Collection: { "data": [...], "meta": { "total": N, "page": 1, "page_size": 20 } }

## Authentication
All endpoints require auth middleware unless annotated: // PUBLIC

Extract member ID from JWT claims via auth middleware context.
Never re-parse the token in handlers.

## Handler pattern (always follow)
1. Parse and validate input (@internal/validator/validator.go)
2. Extract claims from context (set by auth middleware)
3. Call service method
4. Map service errors to HTTP status codes
5. Return response (@internal/response/response.go)

No business logic in handlers. Handlers are thin HTTP adapters only.
```

### git.md

```markdown
# Git rules

## Branches
- Create a feature branch before any changes: git checkout -b feat/PROJ-[id]-description
- Never commit to main, master, or develop
- One branch per ticket — no mixing of unrelated changes

## Commit format (conventional commits)
type(scope): description [PROJ-ID]

Types: feat  fix  docs  test  refactor  chore  perf
Example: feat(booking): add slot hold with Redis TTL [PROJ-42]

Separate commits for tests then implementation:
  test(booking): add failing slot conflict tests [PROJ-42]
  feat(booking): implement slot hold and conflict check [PROJ-42]

## Before committing
Run make done. Never commit if it fails.

## PRs
- Open a PR for every feature
- Description must include: what changed, why, Closes PROJ-[id], make done output
- Auth/payment changes: tag // SECURITY: manual review required in code and PR

## Never
- git push --force to main, master, or develop
- Commit directly to a protected branch
- Skip make done before committing
```

---

<a name="ref-prd"></a>

## Reference: PRD Template

```markdown
# PRD: [Feature Name]
**Feature ID:** [X.X]
**Ticket:** PROJ-[number]
**Branch:** `feat/PROJ-[number]-[name]`
**Phase:** [1 or 2]
**Status:** 📝 Draft | 🔄 In Progress | ✅ Done

---

## Implementation Status
<!-- Claude: check this table at session start.
     Verify each component by checking whether the key file exists.
     Update status and PR as each phase completes. -->

| Component        | Status  | PR | make done |
|---|---|---|---|
| Domain structs   | ⏳ Todo | —  | —         |
| DB migration     | ⏳ Todo | —  | —         |
| Repository layer | ⏳ Todo | —  | —         |
| Service layer    | ⏳ Todo | —  | —         |
| Handler layer    | ⏳ Todo | —  | —         |
| Tests ≥ 80%      | ⏳ Todo | —  | —         |

Status key: ⏳ Todo · 🔄 In Progress · ✅ Done · 🚫 Blocked

---

## Problem statement
[What pain does this solve, and for whom? 2–3 sentences.]

## Goal
[One sentence: what does done look like for the user?]

## Out of scope — do not build in this ticket
- [Item — PROJ-X]

---

## User flows

### Primary: [happy path name]
```

1. [Step]
2. [Step]

```

### Edge: [edge case name]
```

[Edge case flow]

```

---

## Acceptance criteria
```

Given [context]
When  [action]
Then  [expected outcome]

```

---

## Data model

```sql
CREATE TABLE [table_name] (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    [field]    [TYPE] NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- Money: always INTEGER (smallest currency unit). Never FLOAT.
-- Deletes: soft only — use deleted_at TIMESTAMPTZ or a status enum.
```

---

## API endpoints

```
METHOD  /api/v1/[resource]        Description
METHOD  /api/v1/[resource]/:id    Description
```

---

## Key decisions — resolve before implementation

<!-- Engineer fills these in before plan mode begins. Claude does not implement until all are answered. -->

- [ ]  [Decision 1] → ___
- [ ]  [Decision 2] → ___

---

## Files to create / modify

```
New:
  internal/[module]/handler/[feature].go
  internal/[module]/handler/[feature]_test.go
  internal/[module]/service/[feature].go
  internal/[module]/service/[feature]_test.go
  internal/[module]/repository/[feature].go
  internal/[module]/repository/[feature]_test.go
  internal/domain/[feature].go
  db/migrations/00N_create_[table].sql

Modified:
  internal/router/router.go

Do not touch:
  internal/auth/         (out of scope)
```

---

## Non-functional requirements

| Requirement | Target |
| --- | --- |
| Latency | < [N]ms p99 |
| Idempotency | [Yes — key: field_name / No] |

---

## Definition of Done

```
make done   →   make done: OK
```

All ACs covered by tests. Edge cases from `.claude/rules/testing.md` covered.
Coverage ≥ 80% on `internal/[module]/`. Implementation Status table fully updated.
tech-debt.md updated if any shortcuts were taken.

```

---

<a name="ref-techdebt"></a>
## Reference: tech-debt.md

**Who writes it:** Both Claude and engineers. Claude adds entries when making intentional shortcuts (instructed via CLAUDE.md). Engineers add entries during code review or retrospectives.

**Where it lives:** Repo root, alongside `CLAUDE.md` and `PLAN.md`. Committed. Visible to everyone.

**Template:**

```markdown
# Tech Debt Registry
Last updated: [date]

## Priority: Critical (blocks scaling or reliability)
<!-- empty -->

## Priority: High (address next sprint)
- [ ] **TD-001: [Short title]**
  - Impact: [Why it matters and when it will hurt]
  - Source: feat/PROJ-[id] Phase [N]
  - Created: [date]
  - Notes: [Optional — suggested fix approach]

## Priority: Medium (worth fixing, not urgent)
- [ ] **TD-002: [Short title]**
  - Impact: [...]
  - Source: feat/PROJ-[id]
  - Created: [date]

## Priority: Low (nice to have)
- [ ] **TD-003: [Short title]**
  - Impact: [...]
  - Source: feat/PROJ-[id]
  - Created: [date]

## Resolved
- [x] **TD-000: [Short title]**
  - Resolved: [date] in PR #[N]
```

The **Source** field is not optional — without it, entries become orphaned and no one knows what tradeoff was made or why.

---

<a name="ref-tokens"></a>

## Reference: Token Optimization Tools

Both tools are **Claude Code plugins** — they operate at the AI layer, not the code layer.

| Tool | What it does | Install |
| --- | --- | --- |
| **caveman** | Cuts ~65–75% output tokens by compressing prose while keeping all technical detail exact | `curl -fsSL https://raw.githubusercontent.com/JuliusBrussee/caveman/main/install.sh | bash` |
| **ponytail** | Enforces minimal-code discipline: stdlib first, no abstractions, deletion over addition | Inside Claude Code: `/plugin marketplace add DietrichGebert/ponytail` → `/plugin install ponytail@ponytail` → trust hooks → new thread |

**caveman modes:** `/caveman lite` · `/caveman` (default) · `/caveman ultra` · `"normal mode"` to disable

**ponytail commands:** `/ponytail-review` (over-engineering scan of current diff) · `/ponytail ultra` (questions whether the task needs to exist at all)

Combined effect: caveman shrinks what Claude *says*. Ponytail shrinks what Claude *writes*.

---

<a name="ref-card"></a>

## Quick Reference Card

```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
NEW FEATURE — checklist
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
[ ] Write docs/PRD-[feature].md
    — problem statement, ACs, key decisions, Implementation Status table
[ ] git checkout -b feat/PROJ-[id]-description
[ ] Open Claude Code → enter plan mode (Shift+Tab × 2)
[ ] Send Step 1 prompt (explore, read PRD, list files, no changes)
[ ] Receive plan.md → annotate with > NOTE: comments
[ ] Send: "Address all notes in plan.md. Do not implement yet."
[ ] Repeat until zero unresolved notes
[ ] Exit plan mode → implement phase by phase
[ ] After each phase: run make done, update PRD status table, commit
[ ] Open PR → run pre-merge checklist → fresh-session AI review
[ ] Merge after review approval and a green make done

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
NEW SESSION — handoff prompt
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Read CLAUDE.md, PLAN.md, and tech-debt.md.

We are on Phase [N]. Last session completed [what was done].
Continue with [next task].
Only touch files in [scoped path].
Run make done when complete. Show the full output.

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
PROMPT NEEDS MORE CONTEXT
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Resubmit with scope + spec:

"Implement [X].
 Scope: only touch internal/[module]/.
 See @docs/PRD-[feature].md.
 Ticket: PROJ-[id]."

Or enter /plan first.

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
CONTEXT DEGRADING
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
/compact → new session → send handoff prompt

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
TOKEN COMMANDS
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
/caveman ultra        maximum output compression
/ponytail-review      scan diff for over-engineering
/ponytail ultra       question whether the task needs to exist
"normal mode"         disable caveman
```

---

*This playbook is a living document.When Claude Code repeatedly makes the same mistake → update CLAUDE.md.When a process step fails in practice → update this playbook.Review after every 5 completed features.*