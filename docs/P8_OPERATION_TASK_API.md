# P8 Operation Task Backend API

Production Ready: false

P8 Batch 7 exposes the operation-task domain as authenticated backend APIs under the existing Gin `/api/v1` stack. These APIs reuse bearer authentication, trusted tenant context, admin principal loading, existing operationtask services, existing RBAC permissions, response envelopes, and keyset pagination.

## Security boundaries

- `productionReady=false`
- `realCredentialsEnabled=false`
- `realPlatformWriteEnabled=false`
- `automaticPublishEnabled=false`
- `automaticListingEnabled=false`
- `humanConfirmationRequired=true`

Batch 7 does not enable Admin UI, real Douyin calls, real OAuth, real platform credentials, real platform draft writes, automatic publish, automatic listing, background automatic retry, production gray, tag, release, or Production Ready.

## Auth, tenant, and actor

All routes are registered on the existing authenticated `/api/v1` group. Tenant and actor are derived only from trusted backend context:

- tenant: `adminperm.TenantIDFromGin(c)`
- actor: `ctxkey.AdminID`
- role/membership: `adminperm.LoadPrincipal(c, db)`

Request bodies must not provide or override `tenantId`, `actorId`, `reviewerRole`, `isAdmin`, `canApprove`, or `canExecute`. Strict JSON binding rejects unknown fields on write APIs.

## Shop scope (round61)

Operation tasks carry an optional `shopId` binding (`operation_tasks.shop_id`) and are treated as shop-scoped business data, matching the order/procurement/exception scope semantics:

- admin (`AllowedStoreIDs() == nil`) is unrestricted.
- operator/readonly only see tasks bound to granted shops; with no shop grants the task list is empty.
- tasks with `shop_id IS NULL` are tenant-level and admin-only.
- out-of-scope or cross-tenant direct access to a task or any child resource (drafts, approvals, attempts, events) and every write path returns 404 without revealing existence.
- `POST /api/v1/operation-tasks` accepts an optional `shopId`. Admin may omit it (tenant-level task); non-admin roles must bind a granted shop (missing: 400; ungranted or foreign-tenant shop: 404). The bound shop must belong to the authenticated tenant.
- legacy rows are backfilled at migration time from `source_reference` (same-tenant shop id, or product id whose publish links resolve to exactly one shop); non-inferable rows stay tenant-level.

## Routes

### P8-501 Task CRUD / Query API

- `POST /api/v1/operation-tasks`
- `GET /api/v1/operation-tasks`
- `GET /api/v1/operation-tasks/:taskId`
- `POST /api/v1/operation-tasks/:taskId/cancel`

Creation sets server-controlled tenant, creator, initial `suggested` status, revision, timestamps, and a synchronous `task_created` audit event. Cancel is a workflow transition, not arbitrary status mutation. There is no hard delete API.

### P8-502 Draft Edit API

- `POST /api/v1/operation-tasks/:taskId/drafts`
- `PATCH /api/v1/operation-tasks/:taskId/drafts/latest`
- `GET /api/v1/operation-tasks/:taskId/drafts`

Draft APIs use the existing draft version service, canonical payload hashing, expected revision/version checks, and immutable draft history.

### P8-503 Approve / Reject API

- `POST /api/v1/operation-tasks/:taskId/approve`
- `POST /api/v1/operation-tasks/:taskId/reject`

Reviewer identity and role are derived from authenticated context and RBAC. Body-provided reviewer role or approval flags are not accepted.

### P8-504 Execute / Retry API

- `POST /api/v1/operation-tasks/:taskId/execute`
- `POST /api/v1/operation-tasks/:taskId/retry`

Execution and retry use safe adapters and existing production capability guards. Requests cannot enable real writes, credentials, auto publish, auto listing, adapter endpoints, or production capability settings. Responses expose safe attempt/result summaries only and never return `published=true`, `listed=true`, live product IDs, production draft IDs, raw provider errors, tokens, cookies, or credentials.

### P8-505 Attempts / Events API

- `GET /api/v1/operation-tasks/:taskId/attempts`
- `GET /api/v1/operation-tasks/:taskId/events`

History APIs use tenant isolation, audit-read permission, stable ordering, cursor/sequence pagination, and safe metadata redaction. Full payloads and raw idempotency keys are not returned.

## Idempotency and request IDs

High-risk write routes require `Idempotency-Key`:

- task create
- cancel
- draft create/edit
- approve/reject
- execute
- retry

Keys are validated for length and character set. Raw keys are not logged or returned; persisted metadata stores only hashed key material where needed. Request IDs come from trusted request context/header and are not accepted from JSON bodies.

## Pagination

Task and attempt lists use keyset cursors. Event history uses sequence-based pagination. Offset pagination is not used.

## Error mapping

Errors use the existing response envelope with sanitized messages:

- validation errors: HTTP 400
- adapter payload validation failures during execute/retry: HTTP 400 with business code `40001` and `errorCode=execution_validation_failed` (round61; previously a generic HTTP 500/50000). The failed execution attempt is still persisted and finalized before the error is returned.
- adapter-reported permission denials: HTTP 403; adapter state/idempotency conflicts: HTTP 409
- permission and execution-mode denials: HTTP 403
- not found, tenant mismatch, or out-of-shop-scope access: HTTP 404
- revision, state, draft, idempotency, retry, or execution conflicts: HTTP 409
- unexpected failures: HTTP 500 generic internal error

Responses must not expose SQL text, stack traces, raw provider errors, tokens, cookies, credentials, raw idempotency keys, or sensitive payload data.
