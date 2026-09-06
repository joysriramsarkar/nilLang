package data

import (
	"strings"
	"testing"
)

func TestQueryBuilder(t *testing.T) {
	// Postgres dialect ($1, $2)
	sql, args := Table("users").
		Select("id", "email", "status").
		Where("status", "=", "active").
		Where("age", ">=", 18).
		OrderBy("created_at", "DESC").
		Limit(10).
		Offset(20).
		ToSQL()

	expectedSQL := "SELECT id, email, status FROM users WHERE status = $1 AND age >= $2 ORDER BY created_at DESC LIMIT 10 OFFSET 20"
	if sql != expectedSQL {
		t.Errorf("unexpected Postgres SQL:\ngot:  %s\nwant: %s", sql, expectedSQL)
	}
	if len(args) != 2 || args[0] != "active" || args[1] != 18 {
		t.Errorf("unexpected args: %v", args)
	}

	// SQLite dialect (?, ?)
	sqlLite, argsLite := Table("orders").
		WithDriver(DriverSQLite).
		Where("total", ">", 100.50).
		ToSQL()

	if !strings.Contains(sqlLite, "WHERE total > ?") {
		t.Errorf("expected SQLite ? placeholder, got: %s", sqlLite)
	}
	if len(argsLite) != 1 || argsLite[0] != 100.50 {
		t.Errorf("unexpected SQLite args: %v", argsLite)
	}
}

func TestInsertAndUpdateSQL(t *testing.T) {
	record := map[string]interface{}{
		"name":  "Alice",
		"email": "alice@example.com",
	}

	insertSQL, insertArgs := Table("users").InsertSQL(record)
	if !strings.Contains(insertSQL, "INSERT INTO users") || !strings.Contains(insertSQL, "RETURNING *") {
		t.Errorf("invalid insert SQL: %s", insertSQL)
	}
	if len(insertArgs) != 2 {
		t.Errorf("expected 2 insert args, got %d", len(insertArgs))
	}

	updateSQL, updateArgs := Table("users").
		Where("id", "=", 42).
		UpdateSQL(map[string]interface{}{"name": "Bob"})

	if !strings.Contains(updateSQL, "UPDATE users SET name = $1 WHERE id = $2") {
		t.Errorf("invalid update SQL: %s", updateSQL)
	}
	if len(updateArgs) != 2 || updateArgs[0] != "Bob" || updateArgs[1] != 42 {
		t.Errorf("unexpected update args: %v", updateArgs)
	}
}

func TestDBPoolTransactions(t *testing.T) {
	pool := NewDBPool(DBPoolConfig{Driver: DriverPostgres})

	// Success transaction
	err := pool.Transaction(func(tx *Tx) error {
		return nil
	})
	if err != nil {
		t.Errorf("expected successful transaction, got: %v", err)
	}

	// Failed transaction auto-rollbacks
	err = pool.Transaction(func(tx *Tx) error {
		return &customErr{msg: "db failure"}
	})
	if err == nil {
		t.Errorf("expected error from failed transaction")
	}
}

type customErr struct {
	msg string
}

func (e *customErr) Error() string {
	return e.msg
}

func TestMigrations(t *testing.T) {
	runner := NewMigrationRunner()

	runner.Register(Migration{
		Version: 1,
		Name:    "create_users",
		UpSQL:   "CREATE TABLE users (id SERIAL PRIMARY KEY, email TEXT);",
		DownSQL: "DROP TABLE users;",
	}).Register(Migration{
		Version: 2,
		Name:    "create_posts",
		UpSQL:   "CREATE TABLE posts (id SERIAL PRIMARY KEY, title TEXT);",
		DownSQL: "DROP TABLE posts;",
	})

	// Run Up
	applied, err := runner.Up()
	if err != nil || len(applied) != 2 {
		t.Fatalf("Up failed: applied=%v err=%v", applied, err)
	}

	status := runner.Status()
	if len(status) != 2 || status[0].Version != 1 || status[1].Version != 2 {
		t.Errorf("unexpected status: %+v", status)
	}

	// Rollback 1
	downVer, err := runner.Down()
	if err != nil || downVer != 2 {
		t.Fatalf("Down failed: ver=%d err=%v", downVer, err)
	}

	if len(runner.Status()) != 1 {
		t.Errorf("expected 1 applied migration after rollback, got %d", len(runner.Status()))
	}
}

func TestTableCRUDAndAtomicRollback(t *testing.T) {
	pool := NewDBPool(DBPoolConfig{})

	// Insert product
	_, err := pool.Table("products").Insert(pool, map[string]interface{}{
		"id":    "p1",
		"name":  "Milk",
		"stock": 10,
		"price": 50,
	})
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	// Query product
	p, err := pool.Table("products").Where("id", "=", "p1").First(pool)
	if err != nil || p == nil || p["name"] != "Milk" {
		t.Fatalf("query failed: p=%+v, err=%v", p, err)
	}

	// Atomic transaction rollback test
	err = pool.Transaction(func(tx *Tx) error {
		// Deduct stock
		pool.Table("products").Where("id", "=", "p1").Update(pool, map[string]interface{}{"stock": 0})
		// Fail transaction
		return &customErr{msg: "payment gateway timeout"}
	})
	if err == nil {
		t.Fatalf("expected error from failed transaction")
	}

	// Verify stock rolled back to 10!
	pAfter, err := pool.Table("products").Where("id", "=", "p1").First(pool)
	if err != nil || pAfter == nil || pAfter["stock"] != 10 {
		t.Fatalf("stock should have rolled back to 10, got: %+v", pAfter)
	}
}
