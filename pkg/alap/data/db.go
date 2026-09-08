package data

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite" // pure-Go SQLite driver (no CGO required)
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

// OpenPostgres opens a PostgreSQL database connection.
func OpenPostgres(dsn string) (*RealDBPool, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.PingContext(context.Background()); err != nil {
		return nil, fmt.Errorf("ping postgres: %w", err)
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

// Register registers a migration to be run.
func (mr *RealMigrationRunner) Register(m Migration) *RealMigrationRunner {
	mr.mu.Lock()
	defer mr.mu.Unlock()
	mr.migrations = append(mr.migrations, m)
	sort.Slice(mr.migrations, func(i, j int) bool {
		return mr.migrations[i].Version < mr.migrations[j].Version
	})
	return mr
}

// Up creates the schema_migrations table (if needed) and runs all pending migrations.
func (mr *RealMigrationRunner) Up() error {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	// Ensure tracking table exists
	_, err := mr.db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version    INTEGER PRIMARY KEY,
		name       TEXT NOT NULL,
		applied_at TEXT NOT NULL
	)`)
	if err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	for _, m := range mr.migrations {
		var count int
		row := mr.db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version=?", m.Version)
		if err := row.Scan(&count); err != nil {
			return fmt.Errorf("check migration %d: %w", m.Version, err)
		}
		if count > 0 {
			continue // already applied
		}

		// Execute the UP SQL (may contain multiple statements)
		statements := splitStatements(m.UpSQL)
		for _, stmt := range statements {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			if _, err := mr.db.Exec(stmt); err != nil {
				return fmt.Errorf("migration %d (%s) statement error: %w\nSQL: %s", m.Version, m.Name, err, stmt)
			}
		}

		// Record migration
		_, err := mr.db.Exec(
			"INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)",
			m.Version, m.Name, time.Now().UTC().Format(time.RFC3339),
		)
		if err != nil {
			return fmt.Errorf("record migration %d: %w", m.Version, err)
		}
	}
	return nil
}

// Down rolls back the most recent migration.
func (mr *RealMigrationRunner) Down() error {
	mr.mu.Lock()
	defer mr.mu.Unlock()

	var version int
	var name string
	row := mr.db.QueryRow("SELECT version, name FROM schema_migrations ORDER BY version DESC LIMIT 1")
	if err := row.Scan(&version, &name); err != nil {
		return fmt.Errorf("no migrations to rollback: %w", err)
	}

	// Find and run the DownSQL
	for _, m := range mr.migrations {
		if m.Version == version {
			if m.DownSQL == "" {
				return fmt.Errorf("migration %d (%s) has no DownSQL", version, name)
			}
			statements := splitStatements(m.DownSQL)
			for _, stmt := range statements {
				stmt = strings.TrimSpace(stmt)
				if stmt == "" {
					continue
				}
				if _, err := mr.db.Exec(stmt); err != nil {
					return fmt.Errorf("rollback migration %d: %w", version, err)
				}
			}
			break
		}
	}

	_, err := mr.db.Exec("DELETE FROM schema_migrations WHERE version=?", version)
	return err
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
