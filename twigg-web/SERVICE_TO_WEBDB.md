# Migrating legacy services to WebDb

## Context

Legacy pattern: anything that touched the DB was a "service" in `twigg-web/services/`.
Those services `CREATE TABLE IF NOT EXISTS` in their constructor and run raw SQL
through the DEPRECATED `WebDb.Exec/Query/QueryRow` passthroughs.

New pattern:

- **Table setup** happens only in `webdb/migrations/*.sql` (applied by `WebDb`'s
  constructor via `sqlitehelper.Init`).
- **Simple CRUD** of entities lives on `WebDb` (`webdb/aa_public.go`).
- **Services remain only if they do more than CRUD** (hashing, orchestration,
  external APIs, business rules). A surviving service defines its own minimal db
  interface (just the methods it uses) and receives it as a constructor
  dependency; `WebDb` implements it. The service never depends on `webdb.WebDb`
  directly (interfaces at the consumer, per `STYLE.md`).

Already migrated — use as reference implementations:

- Notifications: `webdb/migrations/0000001_notifications.sql`, `webdb/notifications.go`,
  `webdb/notifications_test.go`, entity type in `twigg-web/notification/`.
- Permissions: `webdb/migrations/0000002_permissions.sql`, `webdb/permissions.go`,
  `webdb/service_test.go`, domain types/helpers kept in `twigg-web/permissions/`.
- Review: `webdb/migrations/0000003_reviews.sql`, `webdb/reviews.go` — first
  *surviving service* example: `services/review` keeps the owners/LGTM/status
  logic and defines the minimal `Db` interface it consumes. The entities live
  in `twigg-web/review/` (webdb cannot import `services/review` — cycle via
  `services/user`); consumers import them from there directly. Note the row
  methods take thread types as plain `uint32`.

- Secrets: `webdb/migrations/0000004_secrets.sql`, `webdb/secrets.go`,
  `webdb/secrets_test.go` — surviving service: `services/secrets` keeps the
  AES-GCM encryption, validation and per-repo limit; webdb only ever sees the
  nonce and the ciphertext. The entity lives in `twigg-web/secrets/` (webdb
  cannot import `services/secrets` — its internal tests import webdb) and
  consumers import it from there directly.

- Repos: `webdb/migrations/0000005_repos.sql`, `webdb/repos.go`,
  `webdb/repos_test.go` — surviving service: `services/repo` keeps repo
  creation validation, git mirror url sanitization and the twigg server
  orchestration. The entity lives in `twigg-web/repo/` (webdb cannot import
  `services/repo` — its internal tests import webdb) and consumers import it
  from there directly. The service's Db interface also adapts contexts to the
  twigg server storage views (`WebDb.GetServerRead`/`GetServerWrite`) so the service
  doesn't need `webdb.WebDb.Bind`.

- CI queue: `webdb/migrations/0000006_ci_queue.sql`, `webdb/ciqueue.go`,
  `webdb/ciqueue_test.go` — surviving service: `cicdqueue` keeps the queue
  orchestration, nonces and publisher calls. Its handlers run from the queue
  (not from HTTP muxes), so its Db interface includes `BeginWrite` to open its
  own transactions. No entity to move: rows are primitives; the trigger and
  status strings cross the webdb boundary as plain `string` (like review's
  thread types as plain `uint32`) and the service owns the `CiCdStatus` type.

- Users: `webdb/migrations/0000009_users.sql`, `webdb/users.go`,
  `webdb/stripesubscriptions.go` and their tests — surviving service:
  `services/user` keeps the cli-key and password hashing, the username/email
  validation, the Stripe client orchestration, the state transitions and the
  plan-to-quota/job-limit mapping. The entity had to move out first: it lives in
  `twigg-web/user/` so that `webdb` can return it, and consumers import the
  entity as `user` while aliasing the service `userservice`.

  Two things are worth copying from this one. First, `webdb` methods mirror the
  service's statements one for one (`CreateUser` takes literals, `UpdateUser`
  takes the entity because it writes every field, `SetUserStripeId` writes the
  single column) — an earlier attempt at one `UpsertUser` for both paths was
  rejected for taking a whole entity while using only part of it. Second,
  `webdb` fills the quota fields itself, in the single-row reads and in the
  `GetAllUsers` iterator, so the service no longer stitches a user together
  after reading it; `UserQuotaOwnerName` is exposed so no consumer formats the
  quota key.

Still legacy (create tables in their constructor): `services/jobs`,
`services/trackqueue`.

Out of scope: anything on a separate database (only the main WebDb sqlite db is
being consolidated). Metrics was such a case and was removed instead of migrated
(the current `metrics/` is a rewrite on its own db and stays out of scope).

## Rules

- **One migration commit per service, plus a cleanup commit.** The migration
  commit adds the `WebDb` methods, deletes the service implementation, and
  adapts the service's existing test — in place — to test `WebDb` instead.
  A stacked follow-up commit then moves the adapted test file to `webdb/`
  (step 9). Splitting adapt from move keeps both diffs verifiable: content
  changes are visible in the first, the second is a pure move.
- **Keep the tests.** Do not rewrite scenarios or assertions; only mechanically
  adapt setup and call sites. The reviewer must be able to see the test file survived.
- New code must not call the DEPRECATED `Exec/QueryRow/Query` passthroughs; every
  migration should reduce their call-site count.

## Steps (per service)

### 1. Inventory the service's DB surface

List the tables it creates and every method that is plain CRUD. Decide what
survives: if nothing but CRUD remains, the whole package gets deleted; if real
logic remains (e.g. `user`'s cli-key hashing or Stripe flows), the service stays
but loses its SQL.

### 2. Move the schema to a migration file

Create the next-numbered file in `webdb/migrations/` (e.g. `0000003_users.sql`).
Copy the `CREATE TABLE` / `CREATE INDEX` statements out of the service constructor
verbatim. Keep `IF NOT EXISTS`: production databases already have these tables
(created by the legacy service), so the migration must be a no-op there.

### 3. Decide where the entity type lives

`webdb` must be able to return the entity, so it can never live inside `webdb`
consumers' packages. Move it to a top-level package in twigg-web named after
it, like `twigg-web/notification/`, `twigg-web/repo/` or `twigg-web/user/` — entity
struct + pure helpers, no DB code. This holds even when the service survives:
its own internal tests import `webdb`, which cycles if the entity stayed in the
service package. Plain data struct rules apply (`NewXXX` constructor, no impl
split).

When the entity is widely used, move it in its own stacked commit *before* the
migration, and split that move up:

1. Turn any method that must outlive the move into a function taking the entity
   (`SetUsername(u *User, ...)`), so it can stay in the service package —
   methods can only be declared where the type is.
2. Move the type behind a temporary alias in the service (`type User =
   user.User`), so that commit touches no consumer at all.
3. Repoint consumers a few files per commit: the entity takes the natural name
   (`user`) and the service gets aliased (`userservice`).
4. Delete the alias.

### 4. Implement the CRUD on webDb

New file `webdb/<entity>.go`, methods on the private `webDb` struct, SQL moved
from the service. Follow the local conventions (see `webdb/notifications.go`):

- Context param comes first and is named for the transaction it needs:
  `writeCtx` for writes, `readCtx`/`ctx` for reads. Never open a transaction —
  the caller's context carries it.
- Not-found is `(zeroValue, isNotFoundErr bool, err)` with `err = ErrNotFound`,
  or plain `ErrNotFound`, matching the neighbors.
- Validate required inputs up front (`fmt.Errorf("missing userId")`).
- Multi-row results return `iterator.I[T]` via a small `*sql.Rows` wrapper.

### 5. Add forwarders in aa_public.go

One-line forwarding methods on `WebDb` with a doc comment each. Add `Ctx`
forwarders only if call sites actually go through `WebDb.Bind` (permissions did,
notifications didn't).

### 6. Adapt the service's test to test WebDb — in place

Adapt `services/<x>/service_test.go` where it is; do NOT move the file yet
(moving happens in a follow-up commit, step 9). Change the package to an
external test package and point it at `webdb`. Replace service setup with:

```go
b, cl, err := webdb.NewMem()
// ...
w, closeW, _, err := b.BeginWrite()
defer closeW()
```

and replace `service.Method(...)` with `b.Method(w, ...)`. Everything else —
scenarios, expected values, test names — stays.

### 7. Update call sites and delete the service

- Handlers/services that consumed the old service now call the `WebDb` methods.
  Per `STYLE.md`, new code defines the minimal DB interface it needs instead of
  depending on `webdb.WebDb` directly.
- Remove the service wiring from `main.go` / `server/`.
- Delete the service implementation. If the service survives (non-CRUD logic),
  delete only its SQL: constructor no longer takes a write context for table
  setup, persistence goes through its DB interface.
- If the whole package is deleted, the adapted test file stays behind in
  `services/<x>/` for now — it's removed by the move in step 9.

### 8. Verify and commit

```bash
cd twigg-web && make test   # plus make test-all if shared code moved
tw commit "webdb: migrate <service> CRUD from services/<x>"
```

One service, one commit. Stack the next service's commit on top.

### 9. Cleanup commit: move the tests to webdb

Only after the migration commit is done and tested, move the adapted test file
to its final home: `services/<x>/service_test.go` → `webdb/<entity>_test.go`,
package `webdb_test`. Content changes verbatim-only (package clause, imports if
paths force it) — no scenario, assertion, or naming changes.

This is a separate stacked commit on purpose: the migration commit shows the
test's content changing in place (easy to verify nothing was lost), and this
commit is a pure move (easy to verify nothing changed). Doing both at once
makes the diff show delete+add of the whole file, hiding content changes.

```bash
tw commit "move <service> tests to webdb package"
```