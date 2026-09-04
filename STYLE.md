# STYLE.md

This document explains recurring patterns you'll run into while reading the Go
code in this monorepo.

## General

### aa_public.go: the public surface of a package

Every package should have an `aa_public.go` file.
Its job is to be the *only file you need to read* to understand what a package
exposes and how to use it. All exported types, interfaces,
constructors (`New`/`NewXxx`), and constants live there.

Any function longer than a few lines should live in a private implementation
in a separate file.

We've been following this pattern to force us to be very mindfull of
what **needs** to be public and so that we can always quickly see what a
package is all about.

### Public struct wrapping a private implementation

To try to enforce the use of constructors, public structs embed a private
struct that actually contains all the method implementations.

Packages should also provide a `NewXXXX` to construct a struct `XXXX`.

Example:

```go
// twigg-web/services/owners/aa_public.go
type Service struct {
    s service
}

func (s Service) OwnersLgmtIsOk(repoId uint64, commitId uint64,
    usersWhoLgtmd []string, commitIdToReadOwners uint64,
    supremeLeaders []string, r context.Context) (bool, error) {
    return s.s.OwnersLgmtIsOk(repoId, commitId, usersWhoLgtmd, commitIdToReadOwners, supremeLeaders, r)
}

func New(repo ServerProvider) Service {
    return Service{service{sp: repo}}
}
```

### Define interfaces at the consumer. Accept interfaces, return structs.

Following standard go style, packages should in almost all cases declare the
interfaces that they need and take them as arguments. The should have functions
that return struncts and not interfaces. There are, of course, exceptions; but
we try to follow this everywhere.

### Legacy

There's a lot of legacy code that was following a different pattern that I,
partially due to skill issue partially due to missing C++ constructors, thought
would be good.

These implementations define a top-level interface in a package and some constructor
that returns it. We learned the hard way that this is actually not a good idea and
are deprecating this pattern little by little.

## Web Servers

### Handler contexts (overview)

`twigg-web` handlers don't operate directly on `*http.Request` /
`http.ResponseWriter`. Requests pass through a stack of **mux wrappers**
(`twigg-web/wrappers/aa_public.go`), each of which resolves one more layer of
context before calling the next, and each of which defines its own
`XxxMuxRequest` struct that embeds `*http.Request` and adds the fields that
layer resolved:

```
Mux            → raw http.ResponseWriter / *http.Request
  ↳ RlMux      → adds rate limiting
    ↳ AuthMux  → adds Username, UserId, feature Flags   (AuthMuxRequest)
      ↳ UserMux            → adds the loaded user.User  (UserMuxRequest)
        ↳ UserWithSubMux   → adds subscription checks   (UserWithSubMuxRequest)
          ↳ UserRepoMux    → adds Repo + permission      (UserRepoMuxRequest)
```

The idea is that this allows us to define handlers that have typed request shapes.

Two more things worth knowing at a glance:

- **`HandleFuncR` vs `HandleFuncW`**: `R` registers a read-only handler, `W` a
  write handler. Each hands the handler a `context.Context` (`dbRead` /
  `dbWrite`) that isn't just for cancellation — it's bound to a database
  transaction (see `WebDb.BeginRead` / `WebDb.BeginWrite` in
  `twigg-web/webdb/aa_public.go`). The mux opens the transaction, calls the
  handler, and then closes it — committing automatically for `HandleFuncW` if
  the handler's return value says it should. This is also why you'll see
  service methods take a plain `context.Context` parameter named `r` (read) or
  `w` (write) instead of a more specific type — it's the same transaction
  handle threaded through.

### DB Transactions injected in the context

Following the "accept interfaces, return structs" works amazingly well for
most things, but that pattern doesn't adapt well when we're writing transactional
methods.

There are many solutions. IMO all of them are terrible. We chose the one that is
less terrible: DB implementations have a method that create a context with a
transaction in it; and all DB methods receive a context. I know about the dogma
that contexts should contain request-scoped metadata yadayada; but I am chosing to ignore it.
From all the patterns I've tried this is the one that works the best.

Example:
```go
// Returns a write context that must be used with all other methods that write to the db
func (db WebDb) BeginWrite() (writeCtx context.Context, closeTx func(), commitTx func() error, err error) {
	return db.db.BeginWrite()
}
...
func (db WebDb) GetRepoNextLocalId(ctx context.Context, repoId uint64) (n uint64, isNotFoundErr bool, e error) {
	return db.db.GetRepoNextLocalId(ctx, repoId)
}
```

### DB struct should have all entity CRUD methods
Web servers should have a package with a concrete db implementation (i.e. a struct)
for the supported "driver" (SQLite/Postgres/etc). They should have low level CRUD
methods for all entities of interest.

Handlers and auxiliary services should define the DB interface they need.

Example:
```go
// Method in a concrete db implementation
func (db WebDb) CreateNotification(writeCtx context.Context, userId int64, message string, assetPath string) error {
	return db.db.CreateNotification(writeCtx, userId, message, assetPath)
}

...

// Handler somewhere else defines the interface they need
type Db interface {
	CreateNotification(writeCtx context.Context, userId int64, message string, assetPath string) error
}
```

### Legacy
Saddly, a lot of legacy code is not currently implementing the `DB struct should have all entity CRUD methods`
style. There are many services that receive a whole DB instance.
This is a bad pattern and we're progresivelly deprecating the cases that are still left.

