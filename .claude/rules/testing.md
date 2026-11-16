# Testing Rules

## Mandatory TDD Workflow — No Exceptions

1. Write tests BEFORE writing production code.
2. Run tests after writing them. Confirm they FAIL. Show the failure output.
3. Only then write production code.
4. Implement until all tests pass.
5. Commit tests and implementation in separate commits:
   `test(<scope>): add <description>`
   `feat(<scope>): implement <description>`

## Test Coverage: Beyond Acceptance Criteria

When given acceptance criteria (AC), cover every AC first. Then explore:

- **Null / empty / zero inputs** — missing required fields, empty strings, zero amounts
- **Boundary values** — min/max durations, edge cases (midnight, DST), max page size
- **Error and failure paths** — DB down, external service timeouts, webhook unreachable
- **Idempotency** — same webhook received twice, same request sent twice
- **Concurrency** — two users performing the same action simultaneously (use `-race` flag)
- **Permission edges** — user accessing another user's resources, unauthenticated access
- **State transitions** — test valid status progressions and illegal transitions
- **Integration boundaries** — how does this interact with the adjacent module?

List additional scenarios you identified and why. Flag any that need clarification before implementing. Confirm with the engineer if any discovered scenario needs clarification before implementing.

## Test Naming Convention

```go
// Format: Test[Function]_[Scenario]_[ExpectedOutcome]
func TestCreateUser_DuplicateEmail_Returns409(t *testing.T)
func TestHoldSlot_SlotAlreadyHeld_Returns409(t *testing.T)
func TestGetItems_AllItemsFull_ReturnsEmptyList(t *testing.T)
```

## Layer-Specific Rules

**Repository tests:**
- Use real test DB via your testutil helpers — never mock the DB layer
- Seed data with a fixtures pattern
- Clean up after each test (use `t.Cleanup`)
- Tag integration tests: `//go:build integration` — they are not part of `make done`; run them locally with `make test-integration` (needs `make db-up`)

**Service tests:**
- Mock the repository interface — do not hit the DB
- Mock external services (HTTP APIs, message queues) — do not make real API calls
- Test each business rule as an isolated case

**Handler tests:**
- Use `httptest.NewRecorder()` — never spin up a real server
- Mock the service layer
- Test every error → HTTP status mapping
- Test that auth middleware rejects unauthenticated requests (401)

## No Mocks Without Reason

Prefer real implementations with test databases for repository tests.
Mocks are for external services (HTTP, caches in service tests) and the layer above (service mocks in handler tests).

## Coverage Target

≥ 80% coverage on all internal packages before declaring a feature done.
Run: `go test ./... -cover` and include coverage output in your done report.