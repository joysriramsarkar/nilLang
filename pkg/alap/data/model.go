package data

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Model is the base interface that any typed entity must implement
// to be managed by TypedRepository.
type Model interface {
	TableName() string
	PrimaryKey() (string, interface{})
	ToMap() map[string]interface{}
	FromMap(m map[string]interface{}) error
}

// RelationKind specifies the relationship cardinality.
type RelationKind string

const (
	RelHasOne     RelationKind = "HAS_ONE"
	RelHasMany    RelationKind = "HAS_MANY"
	RelBelongsTo  RelationKind = "BELONGS_TO"
	RelManyToMany RelationKind = "MANY_TO_MANY"
)

// RelationDef describes a relationship between models for preloading.
type RelationDef struct {
	Name        string
	Kind        RelationKind
	ForeignKey  string // foreign key column on target or self
	TargetTable string
	TargetPK    string
	// Assign assigns matching target rows to parent instance
	Assign func(parent Model, relatedRows []map[string]interface{})
}

// TypedRepository provides type-safe, generic persistence and querying over RealDBPool.
type TypedRepository[T Model] struct {
	pool       *RealDBPool
	factory    func() T
	tableName  string
	primaryKey string
	softDelete bool
	timestamps bool
	optimistic bool
	relations  map[string]RelationDef
}

// NewTypedRepository creates a repository for model T.
func NewTypedRepository[T Model](pool *RealDBPool, factory func() T) *TypedRepository[T] {
	instance := factory()
	pkCol, _ := instance.PrimaryKey()
	if pkCol == "" {
		pkCol = "id"
	}
	return &TypedRepository[T]{
		pool:       pool,
		factory:    factory,
		tableName:  instance.TableName(),
		primaryKey: pkCol,
		softDelete: false,
		timestamps: true,
		optimistic: false,
		relations:  make(map[string]RelationDef),
	}
}

// EnableSoftDelete enables soft deletes (requires deleted_at column).
func (r *TypedRepository[T]) EnableSoftDelete() *TypedRepository[T] {
	r.softDelete = true
	return r
}

// EnableOptimisticLock enables optimistic concurrency (requires version column).
func (r *TypedRepository[T]) EnableOptimisticLock() *TypedRepository[T] {
	r.optimistic = true
	return r
}

// DefineRelation registers relationship metadata for Preload.
func (r *TypedRepository[T]) DefineRelation(rel RelationDef) *TypedRepository[T] {
	r.relations[rel.Name] = rel
	return r
}

// ─── QUERY BUILDER WRAPPER ──────────────────────────────────────────────────

// TypedQuery represents an active chainable query for model T.
type TypedQuery[T Model] struct {
	repo      *TypedRepository[T]
	tx        *RealTx
	wheres    []whereClause
	orderBys  []string
	limitVal  int
	offsetVal int
	preloads  []string
	withTrash bool
}

// Query creates a new query on the repository.
func (r *TypedRepository[T]) Query() *TypedQuery[T] {
	return &TypedQuery[T]{
		repo:     r,
		wheres:   make([]whereClause, 0),
		orderBys: make([]string, 0),
		preloads: make([]string, 0),
	}
}

// WithTx binds the query to a transaction.
func (r *TypedRepository[T]) WithTx(tx *RealTx) *TypedQuery[T] {
	q := r.Query()
	q.tx = tx
	return q
}

// WithTx binds this query to a transaction.
func (q *TypedQuery[T]) WithTx(tx *RealTx) *TypedQuery[T] {
	q.tx = tx
	return q
}

// Where adds a filtering condition.
func (q *TypedQuery[T]) Where(col, op string, val interface{}) *TypedQuery[T] {
	q.wheres = append(q.wheres, whereClause{column: col, op: op, val: val})
	return q
}

// OrderBy sets ordering direction.
func (q *TypedQuery[T]) OrderBy(col, dir string) *TypedQuery[T] {
	q.orderBys = append(q.orderBys, fmt.Sprintf("%s %s", col, strings.ToUpper(dir)))
	return q
}

// Limit sets row limit.
func (q *TypedQuery[T]) Limit(limit int) *TypedQuery[T] {
	q.limitVal = limit
	return q
}

// Offset sets rows to skip.
func (q *TypedQuery[T]) Offset(offset int) *TypedQuery[T] {
	q.offsetVal = offset
	return q
}

// Preload queues a relation for batch eager loading (prevents N+1 queries).
func (q *TypedQuery[T]) Preload(relationName string) *TypedQuery[T] {
	q.preloads = append(q.preloads, relationName)
	return q
}

// WithTrashed includes soft-deleted records.
func (q *TypedQuery[T]) WithTrashed() *TypedQuery[T] {
	q.withTrash = true
	return q
}

// ─── QUERY EXECUTION ────────────────────────────────────────────────────────

func (q *TypedQuery[T]) toSQL(forCount bool) (string, []interface{}) {
	driver := q.repo.pool.driver
	var sb strings.Builder
	args := make([]interface{}, 0)
	argIndex := 1

	if forCount {
		sb.WriteString(fmt.Sprintf("SELECT COUNT(*) FROM %s", q.repo.tableName))
	} else {
		sb.WriteString(fmt.Sprintf("SELECT * FROM %s", q.repo.tableName))
	}

	wheres := make([]whereClause, len(q.wheres))
	copy(wheres, q.wheres)

	// Apply soft delete filter unless WithTrashed is requested
	if q.repo.softDelete && !q.withTrash {
		wheres = append(wheres, whereClause{column: "deleted_at", op: "IS", val: nil})
	}

	if len(wheres) > 0 {
		sb.WriteString(" WHERE ")
		for i, w := range wheres {
			if i > 0 {
				sb.WriteString(" AND ")
			}
			if w.val == nil && (w.op == "IS" || w.op == "=") {
				sb.WriteString(fmt.Sprintf("%s IS NULL", w.column))
				continue
			} else if w.val == nil && (w.op == "IS NOT" || w.op == "!=") {
				sb.WriteString(fmt.Sprintf("%s IS NOT NULL", w.column))
				continue
			}

			var ph string
			if driver == DriverPostgres {
				ph = fmt.Sprintf("$%d", argIndex)
			} else {
				ph = "?"
			}
			sb.WriteString(fmt.Sprintf("%s %s %s", w.column, w.op, ph))
			args = append(args, w.val)
			argIndex++
		}
	}

	if !forCount {
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
	}

	return sb.String(), args
}

func (q *TypedQuery[T]) execRows(sqlStr string, args []interface{}) ([]map[string]interface{}, error) {
	var rows *sql.Rows
	var err error
	if q.tx != nil {
		rows, err = q.tx.Query(sqlStr, args...)
	} else {
		rows, err = q.repo.pool.Query(sqlStr, args...)
	}
	if err != nil {
		return nil, fmt.Errorf("exec query [%s]: %w", sqlStr, err)
	}
	defer rows.Close()
	return scanRows(rows)
}

// All executes the query and returns all matching typed models.
func (q *TypedQuery[T]) All() ([]T, error) {
	sqlStr, args := q.toSQL(false)
	rawRows, err := q.execRows(sqlStr, args)
	if err != nil {
		return nil, err
	}

	results := make([]T, len(rawRows))
	for i, row := range rawRows {
		model := q.repo.factory()
		if err := model.FromMap(row); err != nil {
			return nil, fmt.Errorf("hydrate model at index %d: %w", i, err)
		}
		results[i] = model
	}

	// Batch eager load preloads (single batch query per preload, 0 N+1)
	if len(results) > 0 && len(q.preloads) > 0 {
		if err := q.executePreloads(results); err != nil {
			return nil, fmt.Errorf("preload relations: %w", err)
		}
	}

	return results, nil
}

// First returns the first matching model, or false if not found.
func (q *TypedQuery[T]) First() (T, bool, error) {
	q.Limit(1)
	list, err := q.All()
	if err != nil {
		var zero T
		return zero, false, err
	}
	if len(list) == 0 {
		var zero T
		return zero, false, nil
	}
	return list[0], true, nil
}

// Find looks up a record by primary key.
func (q *TypedQuery[T]) Find(id interface{}) (T, bool, error) {
	return q.Where(q.repo.primaryKey, "=", id).First()
}

// FindBy looks up a record by an arbitrary column and value.
func (q *TypedQuery[T]) FindBy(col string, val interface{}) (T, bool, error) {
	return q.Where(col, "=", val).First()
}

// Count returns total matching rows.
func (q *TypedQuery[T]) Count() (int64, error) {
	sqlStr, args := q.toSQL(true)
	var count int64
	var err error
	if q.tx != nil {
		err = q.tx.QueryRow(sqlStr, args...).Scan(&count)
	} else {
		err = q.repo.pool.QueryRow(sqlStr, args...).Scan(&count)
	}
	if err != nil {
		return 0, fmt.Errorf("count query [%s]: %w", sqlStr, err)
	}
	return count, nil
}

// Paginate performs pagination and returns (items, totalCount, error).
func (q *TypedQuery[T]) Paginate(page, pageSize int) ([]T, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	total, err := q.Count()
	if err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	q.Limit(pageSize).Offset(offset)

	items, err := q.All()
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// executePreloads performs batch IN queries to load relationships without N+1.
func (q *TypedQuery[T]) executePreloads(models []T) error {
	for _, relName := range q.preloads {
		relDef, ok := q.repo.relations[relName]
		if !ok {
			return fmt.Errorf("undefined relation %q on %s", relName, q.repo.tableName)
		}

		// Collect parent IDs
		parentIDs := make([]interface{}, len(models))
		idSet := make(map[interface{}]bool)
		uniqueIDs := make([]interface{}, 0)
		for i, m := range models {
			_, pk := m.PrimaryKey()
			parentIDs[i] = pk
			if !idSet[pk] {
				idSet[pk] = true
				uniqueIDs = append(uniqueIDs, pk)
			}
		}

		if len(uniqueIDs) == 0 {
			continue
		}

		// Build batch IN query
		driver := q.repo.pool.driver
		placeholders := make([]string, len(uniqueIDs))
		for i := range uniqueIDs {
			if driver == DriverPostgres {
				placeholders[i] = fmt.Sprintf("$%d", i+1)
			} else {
				placeholders[i] = "?"
			}
		}

		childSQL := fmt.Sprintf(
			"SELECT * FROM %s WHERE %s IN (%s)",
			relDef.TargetTable, relDef.ForeignKey, strings.Join(placeholders, ", "),
		)

		var rows *sql.Rows
		var err error
		if q.tx != nil {
			rows, err = q.tx.Query(childSQL, uniqueIDs...)
		} else {
			rows, err = q.repo.pool.Query(childSQL, uniqueIDs...)
		}
		if err != nil {
			return fmt.Errorf("preload batch query [%s]: %w", childSQL, err)
		}
		defer rows.Close()

		childRows, err := scanRows(rows)
		if err != nil {
			return err
		}

		// Group child rows by foreign key
		grouped := make(map[interface{}][]map[string]interface{})
		for _, crow := range childRows {
			fkVal := crow[relDef.ForeignKey]
			// Normalize string/numeric equality
			fkKey := fmt.Sprintf("%v", fkVal)
			grouped[fkKey] = append(grouped[fkKey], crow)
		}

		// Assign to parents
		if relDef.Assign != nil {
			for _, m := range models {
				_, pk := m.PrimaryKey()
				pkKey := fmt.Sprintf("%v", pk)
				relDef.Assign(m, grouped[pkKey])
			}
		}
	}
	return nil
}

// ─── MUTATION OPERATIONS ────────────────────────────────────────────────────

// Create inserts a new model into the database.
func (r *TypedRepository[T]) Create(entity T) error {
	return r.CreateTx(nil, entity)
}

// CreateTx inserts a model within an existing transaction.
func (r *TypedRepository[T]) CreateTx(tx *RealTx, entity T) error {
	rec := entity.ToMap()
	now := time.Now().UTC().Format(time.RFC3339)

	if r.timestamps {
		if _, ok := rec["created_at"]; !ok || rec["created_at"] == "" || rec["created_at"] == nil {
			rec["created_at"] = now
		}
		rec["updated_at"] = now
	}
	if r.optimistic {
		rec["version"] = 1
	}

	qb := Table(r.tableName).WithDriver(r.pool.driver)
	sqlStr, args := qb.InsertSQL(rec)

	var err error
	if tx != nil {
		_, err = tx.Exec(sqlStr, args...)
	} else {
		_, err = r.pool.Exec(sqlStr, args...)
	}
	if err != nil {
		return fmt.Errorf("create %s: %w", r.tableName, err)
	}

	// Re-hydrate generated fields into model
	_ = entity.FromMap(rec)
	return nil
}

// Update updates an existing model. If optimistic locking is enabled, checks version.
func (r *TypedRepository[T]) Update(entity T) error {
	return r.UpdateTx(nil, entity)
}

// UpdateTx updates an existing model within a transaction.
func (r *TypedRepository[T]) UpdateTx(tx *RealTx, entity T) error {
	rec := entity.ToMap()
	pkCol, pkVal := entity.PrimaryKey()
	if pkVal == nil || pkVal == "" {
		return fmt.Errorf("cannot update %s without primary key value", r.tableName)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if r.timestamps {
		rec["updated_at"] = now
	}

	currentVer, hasVer := rec["version"].(int)
	if !hasVer {
		if v64, ok := rec["version"].(int64); ok {
			currentVer = int(v64)
			hasVer = true
		}
	}

	qb := Table(r.tableName).WithDriver(r.pool.driver).Where(pkCol, "=", pkVal)

	if r.optimistic && hasVer {
		rec["version"] = currentVer + 1
		qb.Where("version", "=", currentVer)
	}

	delete(rec, pkCol) // don't set primary key in SET clause
	sqlStr, args := qb.UpdateSQL(rec)

	var res sql.Result
	var err error
	if tx != nil {
		res, err = tx.Exec(sqlStr, args...)
	} else {
		res, err = r.pool.Exec(sqlStr, args...)
	}
	if err != nil {
		return fmt.Errorf("update %s: %w", r.tableName, err)
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		if r.optimistic && hasVer {
			return fmt.Errorf("optimistic locking conflict on %s (id=%v, version=%d)", r.tableName, pkVal, currentVer)
		}
		return fmt.Errorf("%s with id %v not found", r.tableName, pkVal)
	}

	// Re-hydrate updated fields
	rec[pkCol] = pkVal
	_ = entity.FromMap(rec)
	return nil
}

// Delete permanently removes a record by primary key.
func (r *TypedRepository[T]) Delete(id interface{}) error {
	return r.DeleteTx(nil, id)
}

// DeleteTx permanently removes a record within a transaction.
func (r *TypedRepository[T]) DeleteTx(tx *RealTx, id interface{}) error {
	qb := Table(r.tableName).WithDriver(r.pool.driver).Where(r.primaryKey, "=", id)
	sqlStr, args := qb.deleteSQL()

	var err error
	if tx != nil {
		_, err = tx.Exec(sqlStr, args...)
	} else {
		_, err = r.pool.Exec(sqlStr, args...)
	}
	if err != nil {
		return fmt.Errorf("delete %s: %w", r.tableName, err)
	}
	return nil
}

// SoftDelete sets deleted_at timestamp instead of removing the row.
func (r *TypedRepository[T]) SoftDelete(id interface{}) error {
	return r.SoftDeleteTx(nil, id)
}

// SoftDeleteTx sets deleted_at timestamp within a transaction.
func (r *TypedRepository[T]) SoftDeleteTx(tx *RealTx, id interface{}) error {
	now := time.Now().UTC().Format(time.RFC3339)
	qb := Table(r.tableName).WithDriver(r.pool.driver).Where(r.primaryKey, "=", id)
	sqlStr, args := qb.UpdateSQL(map[string]interface{}{
		"deleted_at": now,
		"updated_at": now,
	})

	var err error
	if tx != nil {
		_, err = tx.Exec(sqlStr, args...)
	} else {
		_, err = r.pool.Exec(sqlStr, args...)
	}
	if err != nil {
		return fmt.Errorf("soft delete %s: %w", r.tableName, err)
	}
	return nil
}

// deleteSQL generates DELETE FROM table WHERE ...
func (q *QueryBuilder) deleteSQL() (string, []interface{}) {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("DELETE FROM %s", q.table))
	args := make([]interface{}, 0)
	argIndex := 1

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
