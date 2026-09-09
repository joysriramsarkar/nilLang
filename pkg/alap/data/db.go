package data

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"hash/fnv"
	"sort"
	"strings"
	"sync"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // PostgreSQL driver — registers "pgx" with database/sql
	_ "modernc.org/sqlite"             // pure-Go SQLite driver (no CGO required)
)

// ─── REAL DATABASE POOL ───────────────────────────────────────────────────────

// RealDBPool wraps database/sql with connection pooling and transaction support.
// This is the production-grade replacement for the in-memory DBPool.
type RealDBPool struct {
	db     *sql.DB
	driver DriverType
	dsn    string
	mu     sync.RWMutex
}

// OpenSQLite opens a SQLite database file (or ":memory:" for testing).
func OpenSQLite(dsn string) (*RealDBPool, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite(%s): %w", dsn, err)
	}
	// SQLite best-practice settings
	db.SetMaxOpenConns(1) // SQLite is single-writer; prevent SQLITE_BUSY
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0) // persistent connection

	if err := db.PingContext(context.Background()); err != nil {
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	// Enable WAL mode and foreign keys
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA cache_size=-8000", // 8 MB cache
		"PRAGMA temp_store=MEMORY",
		"PRAGMA mmap_size=268435456", // 256 MB mmap
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			return nil, fmt.Errorf("set pragma %q: %w", p, err)
		}
	}

	return &RealDBPool{db: db, driver: DriverSQLite, dsn: dsn}, nil
}

// OpenPostgres opens a PostgreSQL database connection via the pgx/v5 stdlib adapter.
// DSN format: postgres://user:pass@host:5432/dbname?sslmode=disable
func OpenPostgres(dsn string) (*RealDBPool, error) {
	db, err := sql.Open("pgx", dsn) // "pgx" registered by pgx/v5/stdlib
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(2 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping postgres (%s): %w", dsn, err)
	}
	return &RealDBPool{db: db, driver: DriverPostgres, dsn: dsn}, nil
}

// Ping checks database connectivity.
func (p *RealDBPool) Ping() error {
	return p.db.PingContext(context.Background())
}

// Close closes the database connection pool.
func (p *RealDBPool) Close() error {
	return p.db.Close()
}

// Driver returns the configured database driver type.
func (p *RealDBPool) Driver() DriverType {
	return p.driver
}

// Exec executes a statement that does not return rows (INSERT, UPDATE, DELETE, DDL).
func (p *RealDBPool) Exec(query string, args ...interface{}) (sql.Result, error) {
	return p.db.ExecContext(context.Background(), query, args...)
}

// QueryRow executes a query expected to return a single row.
func (p *RealDBPool) QueryRow(query string, args ...interface{}) *sql.Row {
	return p.db.QueryRowContext(context.Background(), query, args...)
}

// Query executes a query that returns multiple rows.
func (p *RealDBPool) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return p.db.QueryContext(context.Background(), query, args...)
}

// ─── REAL TRANSACTIONS ────────────────────────────────────────────────────────

// RealTx wraps a real *sql.Tx with driver context for SQL generation.
type RealTx struct {
	tx     *sql.Tx
	driver DriverType
}

// Exec executes a statement within this transaction.
func (t *RealTx) Exec(query string, args ...interface{}) (sql.Result, error) {
	return t.tx.ExecContext(context.Background(), query, args...)
}

// QueryRow executes a single-row query within this transaction.
func (t *RealTx) QueryRow(query string, args ...interface{}) *sql.Row {
	return t.tx.QueryRowContext(context.Background(), query, args...)
}

// Query executes a multi-row query within this transaction.
func (t *RealTx) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return t.tx.QueryContext(context.Background(), query, args...)
}

// Commit commits the transaction.
func (t *RealTx) Commit() error { return t.tx.Commit() }

// Rollback rolls back the transaction.
func (t *RealTx) Rollback() error { return t.tx.Rollback() }

// BeginTx starts a real database transaction with the given isolation level.
func (p *RealDBPool) BeginTx(iso sql.IsolationLevel) (*RealTx, error) {
	tx, err := p.db.BeginTx(context.Background(), &sql.TxOptions{Isolation: iso})
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	return &RealTx{tx: tx, driver: p.driver}, nil
}

// Transaction executes fn inside an atomic transaction. Automatically rolls back on error or panic.
func (p *RealDBPool) Transaction(fn func(tx *RealTx) error) (err error) {
	tx, err := p.BeginTx(sql.LevelReadCommitted)
	if err != nil {
		return err
	}
	defer func() {
		if r := recover(); r != nil {
			_ = tx.Rollback()
			err = fmt.Errorf("transaction panic: %v", r)
		} else if err != nil {
			_ = tx.Rollback()
		}
	}()
	if err = fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

// isTransientError reports whether err represents a transient database error
// that is safe to retry. Checks for SQLite BUSY/locked and PostgreSQL
// serialization failure (SQLSTATE 40001) and deadlock errors.
func isTransientError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	// SQLite: database is locked / SQLITE_BUSY / table is locked
	if strings.Contains(msg, "database is locked") ||
		strings.Contains(msg, "SQLITE_BUSY") ||
		strings.Contains(msg, "database table is locked") ||
		strings.Contains(msg, "table is locked") {
		return true
	}
	// PostgreSQL: serialization failure (40001) or deadlock detected (40P01)
	if strings.Contains(msg, "40001") ||
		strings.Contains(msg, "could not serialize access") ||
		strings.Contains(msg, "deadlock detected") ||
		strings.Contains(msg, "40P01") {
		return true
	}
	return false
}

// TransactionWithRetry executes fn inside an ACID transaction, automatically
// retrying up to maxRetries times on transient failures (SQLite BUSY or
// Postgres serialization errors). Retry delays use truncated exponential
// backoff starting at 10ms, capped at 500ms. Non-transient errors are
// returned immediately without retrying.
//
// Use maxRetries=0 for a single attempt with no retry (equivalent to Transaction).
// Recommended value for POS checkout: 5.
func (p *RealDBPool) TransactionWithRetry(fn func(tx *RealTx) error, maxRetries int) error {
	const (
		initialBackoff = 10 * time.Millisecond
		maxBackoff     = 500 * time.Millisecond
	)
	backoff := initialBackoff
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		err := p.Transaction(fn)
		if err == nil {
			return nil
		}
		if !isTransientError(err) {
			return err // non-transient: propagate immediately
		}
		lastErr = err
		if attempt < maxRetries {
			time.Sleep(backoff)
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
	return fmt.Errorf("transaction failed after %d retries: %w", maxRetries, lastErr)
}

// ─── QUERY BUILDER REAL EXECUTION ────────────────────────────────────────────

// RealGet executes the QueryBuilder SELECT against a real DB and returns rows as []map[string]interface{}.
func (q *QueryBuilder) RealGet(pool *RealDBPool) ([]map[string]interface{}, error) {
	sqlStr, args := q.ToSQL()
	rows, err := pool.Query(sqlStr, args...)
	if err != nil {
		return nil, fmt.Errorf("query [%s]: %w", sqlStr, err)
	}
	defer rows.Close()
	return scanRows(rows)
}

// RealGetTx executes SELECT within a transaction.
func (q *QueryBuilder) RealGetTx(tx *RealTx) ([]map[string]interface{}, error) {
	sqlStr, args := q.ToSQL()
	rows, err := tx.Query(sqlStr, args...)
	if err != nil {
		return nil, fmt.Errorf("tx query [%s]: %w", sqlStr, err)
	}
	defer rows.Close()
	return scanRows(rows)
}

// RealFirst returns the first row.
func (q *QueryBuilder) RealFirst(pool *RealDBPool) (map[string]interface{}, error) {
	q.Limit(1)
	rows, err := q.RealGet(pool)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return rows[0], nil
}

// RealFind finds a record by primary key.
func (q *QueryBuilder) RealFind(pool *RealDBPool, id interface{}) (map[string]interface{}, bool, error) {
	q.Where("id", "=", id)
	row, err := q.RealFirst(pool)
	if err != nil {
		return nil, false, err
	}
	return row, row != nil, nil
}

// RealFindBy finds a record by column=value.
func (q *QueryBuilder) RealFindBy(pool *RealDBPool, col string, val interface{}) (map[string]interface{}, bool, error) {
	q.Where(col, "=", val)
	row, err := q.RealFirst(pool)
	if err != nil {
		return nil, false, err
	}
	return row, row != nil, nil
}

// RealInsert inserts a record into the real database.
func (q *QueryBuilder) RealInsert(pool *RealDBPool, record map[string]interface{}) error {
	sqlStr, args := q.WithDriver(pool.driver).InsertSQL(record)
	if pool.driver == DriverPostgres {
		// Remove RETURNING * for simple insert
		sqlStr = strings.Replace(sqlStr, " RETURNING *", "", 1)
	}
	_, err := pool.Exec(sqlStr, args...)
	return err
}

// RealInsertTx inserts a record within a transaction.
func (q *QueryBuilder) RealInsertTx(tx *RealTx, record map[string]interface{}) error {
	sqlStr, args := q.WithDriver(tx.driver).InsertSQL(record)
	if tx.driver == DriverPostgres {
		sqlStr = strings.Replace(sqlStr, " RETURNING *", "", 1)
	}
	_, err := tx.Exec(sqlStr, args...)
	return err
}

// RealUpdate updates matching records in the real database.
func (q *QueryBuilder) RealUpdate(pool *RealDBPool, record map[string]interface{}) (int64, error) {
	sqlStr, args := q.WithDriver(pool.driver).UpdateSQL(record)
	res, err := pool.Exec(sqlStr, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// RealUpdateTx updates matching records within a transaction.
func (q *QueryBuilder) RealUpdateTx(tx *RealTx, record map[string]interface{}) (int64, error) {
	sqlStr, args := q.WithDriver(tx.driver).UpdateSQL(record)
	res, err := tx.Exec(sqlStr, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// RealDelete deletes matching records.
func (q *QueryBuilder) RealDelete(pool *RealDBPool) (int64, error) {
	where, args := q.whereSQL(1)
	sqlStr := fmt.Sprintf("DELETE FROM %s", q.table)
	if where != "" {
		sqlStr += " WHERE " + where
	}
	res, err := pool.Exec(sqlStr, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// RealPaginate retrieves a page of results with total count.
func (q *QueryBuilder) RealPaginate(pool *RealDBPool, page, pageSize int) ([]map[string]interface{}, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	// Count total
	countWhere, countArgs := q.whereSQL(1)
	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM %s", q.table)
	if countWhere != "" {
		countSQL += " WHERE " + countWhere
	}
	var total int64
	if err := pool.QueryRow(countSQL, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Page rows
	q.Limit(pageSize).Offset((page - 1) * pageSize)
	rows, err := q.RealGet(pool)
	return rows, total, err
}

// whereSQL generates the WHERE clause fragment and args, starting argIndex at start.
func (q *QueryBuilder) whereSQL(startIdx int) (string, []interface{}) {
	if len(q.wheres) == 0 {
		return "", nil
	}
	parts := make([]string, 0, len(q.wheres))
	args := make([]interface{}, 0, len(q.wheres))
	argIdx := startIdx
	for _, w := range q.wheres {
		var ph string
		if q.driver == DriverPostgres {
			ph = fmt.Sprintf("$%d", argIdx)
		} else {
			ph = "?"
		}
		parts = append(parts, fmt.Sprintf("%s %s %s", w.column, w.op, ph))
		args = append(args, w.val)
		argIdx++
	}
	return strings.Join(parts, " AND "), args
}

// ─── REAL MIGRATION RUNNER ────────────────────────────────────────────────────

// RealMigrationRunner executes DDL migrations against a real sql.DB.
// It supports both SQLite and PostgreSQL with dialect-aware placeholders.
// On PostgreSQL it acquires a session-level advisory lock so concurrent
// processes (rolling deploys) cannot run migrations simultaneously.
// Each migration runs inside its own transaction: a partial failure rolls
// back only that migration, leaving already-applied ones intact.
type RealMigrationRunner struct {
	db         *RealDBPool
	migrations []Migration
	mu         sync.Mutex
}

// NewRealMigrationRunner creates a runner bound to a real DB pool.
func NewRealMigrationRunner(pool *RealDBPool) *RealMigrationRunner {
	return &RealMigrationRunner{
		db:         pool,
		migrations: make([]Migration, 0),
	}
}

// Register appends a migration and keeps the list sorted by version.
func (mr *RealMigrationRunner) Register(m Migration) *RealMigrationRunner {
	mr.mu.Lock()
	defer mr.mu.Unlock()
	mr.migrations = append(mr.migrations, m)
	sort.Slice(mr.migrations, func(i, j int) bool {
		return mr.migrations[i].Version < mr.migrations[j].Version
	})
	return mr
}

// ph returns the SQL parameter placeholder appropriate for the driver.
// PostgreSQL requires $N (1-indexed); SQLite uses ?.
func (mr *RealMigrationRunner) ph(n int) string {
	if mr.db.driver == DriverPostgres {
		return fmt.Sprintf("$%d", n)
	}
	return "?"
}

// acquireAdvisoryLock acquires a PostgreSQL session-level advisory lock so
// that at most one process runs migrations at a time. It is a no-op on SQLite.
// Returns a release function that must be deferred by the caller.
func (mr *RealMigrationRunner) acquireAdvisoryLock() (func(), error) {
	if mr.db.driver != DriverPostgres {
		return func() {}, nil
	}
	// Stable 32-bit FNV hash of "nilLang-migrations" used as advisory lock key.
	h := fnv.New32a()
	h.Write([]byte("nilLang-migrations"))
	lockID := int64(h.Sum32())

	var acquired bool
	if err := mr.db.QueryRow("SELECT pg_try_advisory_lock($1)", lockID).Scan(&acquired); err != nil {
		return nil, fmt.Errorf("pg_try_advisory_lock: %w", err)
	}
	if !acquired {
		return nil, fmt.Errorf("migration already in progress — pg advisory lock held by another connection")
	}
	return func() {
		_ = mr.db.QueryRow("SELECT pg_advisory_unlock($1)", lockID).Scan(new(bool))
	}, nil
}

// Up creates the schema_migrations table (if needed) and runs all pending
// migrations in version order. Each migration is wrapped in its own
// transaction so a failure never leaves the schema half-applied.
func (mr *RealMigrationRunner) Up() error {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	release, err := mr.acquireAdvisoryLock()
	if err != nil {
		return err
	}
	defer release()

	// Tracking table — same DDL works for SQLite and PostgreSQL.
	_, err = mr.db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version    INTEGER PRIMARY KEY,
		name       TEXT NOT NULL,
		checksum   TEXT NOT NULL DEFAULT '',
		applied_at TEXT NOT NULL
	)`)
	if err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}
	// Migrate legacy schema_migrations without checksum column if needed
	_, _ = mr.db.Exec(`ALTER TABLE schema_migrations ADD COLUMN checksum TEXT NOT NULL DEFAULT ''`)

	for _, m := range mr.migrations {
		currentChecksum := m.ComputeChecksum()
		var existingChecksum string
		err := mr.db.QueryRow(
			fmt.Sprintf("SELECT checksum FROM schema_migrations WHERE version=%s", mr.ph(1)),
			m.Version,
		).Scan(&existingChecksum)

		if err == nil {
			// Already applied: verify checksum to ensure historical migration was not tampered with
			if existingChecksum != "" && existingChecksum != currentChecksum {
				return fmt.Errorf("migration %d (%s) checksum mismatch: database has %q, codebase has %q. Applied migration was modified",
					m.Version, m.Name, existingChecksum, currentChecksum)
			}
			continue
		} else if !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("check migration %d: %w", m.Version, err)
		}

		// Run DDL statements inside an atomic transaction.
		if err := mr.execMigrationTx(m.Version, m.Name, m.UpSQL); err != nil {
			return err
		}

		// Record the applied migration with its checksum.
		if _, err := mr.db.Exec(
			fmt.Sprintf(
				"INSERT INTO schema_migrations (version, name, checksum, applied_at) VALUES (%s, %s, %s, %s)",
				mr.ph(1), mr.ph(2), mr.ph(3), mr.ph(4),
			),
			m.Version, m.Name, currentChecksum, time.Now().UTC().Format(time.RFC3339),
		); err != nil {
			return fmt.Errorf("record migration %d: %w", m.Version, err)
		}
	}
	return nil
}

// CurrentVersion returns the highest applied migration version in the database.
func (mr *RealMigrationRunner) CurrentVersion() (int, error) {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	var maxVersion sql.NullInt64
	err := mr.db.QueryRow("SELECT MAX(version) FROM schema_migrations").Scan(&maxVersion)
	if err != nil {
		return 0, nil
	}
	if !maxVersion.Valid {
		return 0, nil
	}
	return int(maxVersion.Int64), nil
}

// execMigrationTx executes one migration's SQL inside a database transaction.
// On failure the transaction is rolled back and an error is returned.
func (mr *RealMigrationRunner) execMigrationTx(version int, name, sqlStr string) error {
	isoLevel := sql.LevelDefault
	if mr.db.driver == DriverPostgres {
		isoLevel = sql.LevelSerializable
	}
	tx, err := mr.db.BeginTx(isoLevel)
	if err != nil {
		return fmt.Errorf("migration %d begin tx: %w", version, err)
	}
	for _, stmt := range splitStatements(sqlStr) {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := tx.Exec(stmt); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migration %d (%s) failed: %w\nSQL: %s", version, name, err, stmt)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("migration %d (%s) commit: %w", version, name, err)
	}
	return nil
}

// Down rolls back the most recent migration.
func (mr *RealMigrationRunner) Down() error {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	var version int
	var name string
	row := mr.db.QueryRow(
		"SELECT version, name FROM schema_migrations ORDER BY version DESC LIMIT 1",
	)
	if err := row.Scan(&version, &name); err != nil {
		return fmt.Errorf("no migrations to rollback: %w", err)
	}

	for _, m := range mr.migrations {
		if m.Version != version {
			continue
		}
		if m.DownSQL == "" {
			return fmt.Errorf("migration %d (%s) has no DownSQL", version, name)
		}
		if err := mr.execMigrationTx(version, name+" (down)", m.DownSQL); err != nil {
			return err
		}
		break
	}

	_, err := mr.db.Exec(
		fmt.Sprintf("DELETE FROM schema_migrations WHERE version=%s", mr.ph(1)),
		version,
	)
	return err
}

// RollbackN rolls back the N most recent migrations in reverse version order.
func (mr *RealMigrationRunner) RollbackN(n int) error {
	for i := 0; i < n; i++ {
		if err := mr.Down(); err != nil {
			return fmt.Errorf("rollback step %d/%d: %w", i+1, n, err)
		}
	}
	return nil
}

// Status returns applied migrations ordered by version ascending.
// Unlike the in-memory MigrationRunner.Status(), this queries the real DB.
func (mr *RealMigrationRunner) Status() ([]MigrationRecord, error) {
	rows, err := mr.db.Query(
		"SELECT version, name, checksum, applied_at FROM schema_migrations ORDER BY version ASC",
	)
	if err != nil {
		return nil, fmt.Errorf("migration status query: %w", err)
	}
	defer rows.Close()

	var records []MigrationRecord
	for rows.Next() {
		var r MigrationRecord
		var appliedAt string
		if err := rows.Scan(&r.Version, &r.Name, &r.Checksum, &appliedAt); err != nil {
			return nil, err
		}
		r.AppliedAt, _ = time.Parse(time.RFC3339, appliedAt)
		records = append(records, r)
	}
	return records, rows.Err()
}

// splitStatements splits a multi-statement SQL string on semicolons.
func splitStatements(sql string) []string {
	parts := strings.Split(sql, ";")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			result = append(result, p)
		}
	}
	return result
}

// scanRows converts *sql.Rows into []map[string]interface{}.
func scanRows(rows *sql.Rows) ([]map[string]interface{}, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	result := make([]map[string]interface{}, 0)
	for rows.Next() {
		vals := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := make(map[string]interface{}, len(cols))
		for i, col := range cols {
			// SQLite returns []byte for TEXT; convert to string
			if b, ok := vals[i].([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = vals[i]
			}
		}
		result = append(result, row)
	}
	return result, rows.Err()
}
