# Migrations

Tables are created from `models.ALL` only when missing, so a model change never reaches an existing database on its own. A migration does that.

- One Go file per migration, named `YYYYMMDDHHMMSS_short_name.go` (bun reads the name and comment from the file name).
- Each file registers itself in an `init()` with `Migrations.MustRegister(up, down)`.
- `up` must work on a fresh database too, where the model already created the current shape: check before changing (see `20261101000000_tasks_title.go`) and support both SQLite and Postgres via `db.Dialect().Name()`.
- `down` may be a no-op with a comment: we do not roll back schema.
- Startup runs them: `controller.Setup` creates the tables, then `migrateDB` runs `Init` and `Migrate` (bun's `bun_migrations` tables), logs the applied group, and fails startup if one fails. A migration is marked applied only on success, so a failed one retries on the next start.
- `agc migrate run --name <id>` runs a single migration by hand against the same tables.
