package data

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// DriverType defines supported databases
type DriverType string

const (
	DriverPostgres DriverType = "postgres"
	DriverSQLite   DriverType = "sqlite"
)

// QueryBuilder constructs parameterized SQL statements
type QueryBuilder struct {
	driver     DriverType
	table      string
	selected   []string
	wheres     []whereClause
	orderBys   []string
	limitVal   int
	offsetVal  int
	joins      []string
}

type whereClause struct {
	column string
	op     string
	val    interface{}
	isRaw  bool
}

// Table initializes a query builder for a table
func Table(name string) *QueryBuilder {
	return &QueryBuilder{
		driver:    DriverPostgres,
		table:     name,
		selected:  []string{"*"},
		wheres:    make([]whereClause, 0),
		orderBys:  make([]string, 0),
		joins:     make([]string, 0),
	}
}

// WithDriver sets dialect (Postgres or SQLite)
func (q *QueryBuilder) WithDriver(d DriverType) *QueryBuilder {
	q.driver = d
	return q
}

// Select specifies columns to retrieve
func (q *QueryBuilder) Select(cols ...string) *QueryBuilder {
	q.selected = cols
	return q
}

// Where adds a filtering condition
func (q *QueryBuilder) Where(col, op string, val interface{}) *QueryBuilder {
	q.wheres = append(q.wheres, whereClause{column: col, op: op, val: val})
	return q
}

// OrderBy adds ordering
func (q *QueryBuilder) OrderBy(col, dir string) *QueryBuilder {
	q.orderBys = append(q.orderBys, fmt.Sprintf("%s %s", col, strings.ToUpper(dir)))
	return q
}

// Limit sets maximum rows
func (q *QueryBuilder) Limit(limit int) *QueryBuilder {
	q.limitVal = limit
	return q
}

// Offset sets rows to skip
func (q *QueryBuilder) Offset(offset int) *QueryBuilder {
	q.offsetVal = offset
	return q
}

// Join adds an INNER/LEFT JOIN
func (q *QueryBuilder) Join(joinClause string) *QueryBuilder {
	q.joins = append(q.joins, joinClause)
	return q
}

// ToSQL generates SELECT statement with parameterized bindings
func (q *QueryBuilder) ToSQL() (string, []interface{}) {
	var sb strings.Builder
	args := make([]interface{}, 0)
	argIndex := 1

	sb.WriteString("SELECT ")
	sb.WriteString(strings.Join(q.selected, ", "))
	sb.WriteString(" FROM ")
	sb.WriteString(q.table)

	for _, j := range q.joins {
		sb.WriteString(" ")
		sb.WriteString(j)
	}

	if len(q.wheres) > 0 {
		sb.WriteString(" WHERE ")
		for i, w := range q.wheres {
			if i > 0 {
				sb.WriteString(" AND ")
			}
			var placeholder string
			if q.driver == DriverPostgres {
				placeholder = fmt.Sprintf("$%d", argIndex)
			} else {
				placeholder = "?"
			}
			sb.WriteString(fmt.Sprintf("%s %s %s", w.column, w.op, placeholder))
			args = append(args, w.val)
			argIndex++
		}
	}

	if len(q.orderBys) > 0 {
		sb.WriteString(" ORDER BY ")
		sb.WriteString(strings.Join(q.orderBys, ", "))
	}

	if q.limitVal > 0 {
		sb.WriteString(fmt.Sprintf(" LIMIT %d", q.limitVal))
	}

	if q.offsetVal > 0 {
		sb.WriteString(fmt.Sprintf(" OFFSET %d", q.offsetVal))
	}

	return sb.String(), args
}

// InsertSQL generates parameterized INSERT statement
func (q *QueryBuilder) InsertSQL(record map[string]interface{}) (string, []interface{}) {
	cols := make([]string, 0, len(record))
	placeholders := make([]string, 0, len(record))
	args := make([]interface{}, 0, len(record))
	argIndex := 1

	for k, v := range record {
		cols = append(cols, k)
		if q.driver == DriverPostgres {
			placeholders = append(placeholders, fmt.Sprintf("$%d", argIndex))
		} else {
			placeholders = append(placeholders, "?")
		}
		args = append(args, v)
		argIndex++
	}

	sql := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		q.table, strings.Join(cols, ", "), strings.Join(placeholders, ", "))

	if q.driver == DriverPostgres {
		sql += " RETURNING *"
	}

	return sql, args
}

// UpdateSQL generates parameterized UPDATE statement
func (q *QueryBuilder) UpdateSQL(record map[string]interface{}) (string, []interface{}) {
	setClauses := make([]string, 0, len(record))
	args := make([]interface{}, 0)
	argIndex := 1

	for k, v := range record {
		var placeholder string
		if q.driver == DriverPostgres {
			placeholder = fmt.Sprintf("$%d", argIndex)
		} else {
			placeholder = "?"
		}
		setClauses = append(setClauses, fmt.Sprintf("%s = %s", k, placeholder))
		args = append(args, v)
		argIndex++
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("UPDATE %s SET %s", q.table, strings.Join(setClauses, ", ")))

	if len(q.wheres) > 0 {
		sb.WriteString(" WHERE ")
		for i, w := range q.wheres {
			if i > 0 {
				sb.WriteString(" AND ")
			}
			var placeholder string
			if q.driver == DriverPostgres {
				placeholder = fmt.Sprintf("$%d", argIndex)
			} else {
				placeholder = "?"
			}
			sb.WriteString(fmt.Sprintf("%s %s %s", w.column, w.op, placeholder))
			args = append(args, w.val)
			argIndex++
		}
	}

	return sb.String(), args
}

// ─── CONNECTION POOL & TRANSACTION ABSTRACTION ──────────────────────────────

// DBPoolConfig configures connection pool
type DBPoolConfig struct {
	Driver          DriverType    `json:"driver"`
	DSN             string        `json:"dsn"`
	MaxOpenConns    int           `json:"max_open_conns"`
	MaxIdleConns    int           `json:"max_idle_conns"`
	ConnMaxLifetime time.Duration `json:"conn_max_lifetime"`
}

// DBPool manages database connections, tables and transactions
type DBPool struct {
	cfg      DBPoolConfig
	activeTx int
	tables   map[string][]map[string]interface{}
	mu       sync.RWMutex
}

// NewDBPool creates a configured DB pool
func NewDBPool(cfg DBPoolConfig) *DBPool {
	if cfg.MaxOpenConns == 0 {
		cfg.MaxOpenConns = 25
	}
	if cfg.MaxIdleConns == 0 {
		cfg.MaxIdleConns = 5
	}
	if cfg.ConnMaxLifetime == 0 {
		cfg.ConnMaxLifetime = 5 * time.Minute
	}
	return &DBPool{
		cfg:    cfg,
		tables: make(map[string][]map[string]interface{}),
	}
}

// Table returns a QueryBuilder bound to this database pool
func (p *DBPool) Table(name string) *QueryBuilder {
	qb := Table(name)
	if p.cfg.Driver != "" {
		qb.WithDriver(p.cfg.Driver)
	}
	return qb
}

// ExecTableInsert inserts a record into the table and returns it
func (p *DBPool) ExecTableInsert(tableName string, record map[string]interface{}) map[string]interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()

	recCopy := make(map[string]interface{})
	for k, v := range record {
		recCopy[k] = v
	}
	p.tables[tableName] = append(p.tables[tableName], recCopy)
	return recCopy
}

// ExecTableSelect queries the table with filtering conditions
func (p *DBPool) ExecTableSelect(qb *QueryBuilder) []map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	rows, exists := p.tables[qb.table]
	if !exists {
		return []map[string]interface{}{}
	}

	result := make([]map[string]interface{}, 0)
	for _, row := range rows {
		if matchWheres(row, qb.wheres) {
			projected := make(map[string]interface{})
			if len(qb.selected) == 1 && qb.selected[0] == "*" {
				for k, v := range row {
					projected[k] = v
				}
			} else {
				for _, col := range qb.selected {
					if v, ok := row[col]; ok {
						projected[col] = v
					}
				}
			}
			result = append(result, projected)
		}
	}

	if qb.offsetVal > 0 {
		if qb.offsetVal >= len(result) {
			return []map[string]interface{}{}
		}
		result = result[qb.offsetVal:]
	}
	if qb.limitVal > 0 && qb.limitVal < len(result) {
		result = result[:qb.limitVal]
	}

	return result
}

// ExecTableUpdate updates rows matching conditions
func (p *DBPool) ExecTableUpdate(qb *QueryBuilder, updates map[string]interface{}) int64 {
	p.mu.Lock()
	defer p.mu.Unlock()

	rows, exists := p.tables[qb.table]
	if !exists {
		return 0
	}

	var count int64
	for i, row := range rows {
		if matchWheres(row, qb.wheres) {
			for k, v := range updates {
				rows[i][k] = v
			}
			count++
		}
	}
	return count
}

// ExecTableDelete deletes rows matching conditions
func (p *DBPool) ExecTableDelete(qb *QueryBuilder) int64 {
	p.mu.Lock()
	defer p.mu.Unlock()

	rows, exists := p.tables[qb.table]
	if !exists {
		return 0
	}

	filtered := make([]map[string]interface{}, 0)
	var count int64
	for _, row := range rows {
		if matchWheres(row, qb.wheres) {
			count++
		} else {
			filtered = append(filtered, row)
		}
	}
	p.tables[qb.table] = filtered
	return count
}

// Tx represents a database transaction context
type Tx struct {
	ID         string
	committed  bool
	rolledBack bool
	pool       *DBPool
}

// Commit commits the transaction
func (t *Tx) Commit() error {
	if t.rolledBack {
		return fmt.Errorf("cannot commit rolled back transaction")
	}
	t.committed = true
	return nil
}

// Rollback rolls back the transaction
func (t *Tx) Rollback() error {
	if t.committed {
		return fmt.Errorf("cannot rollback committed transaction")
	}
	t.rolledBack = true
	return nil
}

// Transaction executes a function inside an atomic transaction with full rollback on error
func (p *DBPool) Transaction(fn func(tx *Tx) error) (err error) {
	p.mu.Lock()
	p.activeTx++

	// Snapshot all tables for atomic rollback
	snapshot := make(map[string][]map[string]interface{})
	for tbl, rows := range p.tables {
		snapRows := make([]map[string]interface{}, len(rows))
		for i, r := range rows {
			rowCopy := make(map[string]interface{})
			for k, v := range r {
				rowCopy[k] = v
			}
			snapRows[i] = rowCopy
		}
		snapshot[tbl] = snapRows
	}
	p.mu.Unlock()

	tx := &Tx{ID: fmt.Sprintf("tx_%d", time.Now().UnixNano()), pool: p}

	defer func() {
		p.mu.Lock()
		p.activeTx--
		if r := recover(); r != nil {
			p.tables = snapshot
			_ = tx.Rollback()
			err = fmt.Errorf("transaction panic: %v", r)
		} else if err != nil || tx.rolledBack {
			p.tables = snapshot
		}
		p.mu.Unlock()
	}()

	err = fn(tx)
	if err != nil {
		_ = tx.Rollback()
		return err
	}

	if !tx.committed && !tx.rolledBack {
		return tx.Commit()
	}
	return nil
}

// Get executes the query against the database pool and returns matching rows
func (q *QueryBuilder) Get(pool *DBPool) ([]map[string]interface{}, error) {
	if pool == nil {
		return nil, fmt.Errorf("database pool cannot be nil")
	}
	return pool.ExecTableSelect(q), nil
}

// First executes the query and returns the first matching row or nil
func (q *QueryBuilder) First(pool *DBPool) (map[string]interface{}, error) {
	q.Limit(1)
	rows, err := q.Get(pool)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return rows[0], nil
}

// Insert inserts a record into the table
func (q *QueryBuilder) Insert(pool *DBPool, record map[string]interface{}) (map[string]interface{}, error) {
	if pool == nil {
		return nil, fmt.Errorf("database pool cannot be nil")
	}
	return pool.ExecTableInsert(q.table, record), nil
}

// Update updates records matching the query builder's conditions
func (q *QueryBuilder) Update(pool *DBPool, record map[string]interface{}) (int64, error) {
	if pool == nil {
		return 0, fmt.Errorf("database pool cannot be nil")
	}
	return pool.ExecTableUpdate(q, record), nil
}

// Delete deletes records matching the query builder's conditions
func (q *QueryBuilder) Delete(pool *DBPool) (int64, error) {
	if pool == nil {
		return 0, fmt.Errorf("database pool cannot be nil")
	}
	return pool.ExecTableDelete(q), nil
}

func matchWheres(row map[string]interface{}, wheres []whereClause) bool {
	for _, w := range wheres {
		val, ok := row[w.column]
		if !ok {
			return false
		}
		switch w.op {
		case "=":
			if fmt.Sprintf("%v", val) != fmt.Sprintf("%v", w.val) {
				return false
			}
		case "!=":
			if fmt.Sprintf("%v", val) == fmt.Sprintf("%v", w.val) {
				return false
			}
		case ">":
			vFloat, vErr := toFloat(val)
			wFloat, wErr := toFloat(w.val)
			if vErr != nil || wErr != nil || !(vFloat > wFloat) {
				return false
			}
		case "<":
			vFloat, vErr := toFloat(val)
			wFloat, wErr := toFloat(w.val)
			if vErr != nil || wErr != nil || !(vFloat < wFloat) {
				return false
			}
		case ">=":
			vFloat, vErr := toFloat(val)
			wFloat, wErr := toFloat(w.val)
			if vErr != nil || wErr != nil || !(vFloat >= wFloat) {
				return false
			}
		case "<=":
			vFloat, vErr := toFloat(val)
			wFloat, wErr := toFloat(w.val)
			if vErr != nil || wErr != nil || !(vFloat <= wFloat) {
				return false
			}
		}
	}
	return true
}

func toFloat(val interface{}) (float64, error) {
	switch v := val.(type) {
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	default:
		return 0, fmt.Errorf("not numeric")
	}
}

