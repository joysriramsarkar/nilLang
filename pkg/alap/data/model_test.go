package data_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/joysriramsarkar/nilLang/pkg/alap/data"
)

// ─── TEST MODEL DEFINITIONS ──────────────────────────────────────────────────

type CategoryModel struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	CreatedAt string          `json:"created_at"`
	UpdatedAt string          `json:"updated_at"`
	DeletedAt *string         `json:"deleted_at"`
	Products  []*ProductModel `json:"products,omitempty"`
}

func (c *CategoryModel) TableName() string                 { return "categories" }
func (c *CategoryModel) PrimaryKey() (string, interface{}) { return "id", c.ID }
func (c *CategoryModel) ToMap() map[string]interface{} {
	m := map[string]interface{}{
		"id":         c.ID,
		"name":       c.Name,
		"created_at": c.CreatedAt,
		"updated_at": c.UpdatedAt,
	}
	if c.DeletedAt != nil {
		m["deleted_at"] = *c.DeletedAt
	}
	return m
}
func (c *CategoryModel) FromMap(m map[string]interface{}) error {
	if v, ok := m["id"].(string); ok {
		c.ID = v
	}
	if v, ok := m["name"].(string); ok {
		c.Name = v
	}
	if v, ok := m["created_at"].(string); ok {
		c.CreatedAt = v
	}
	if v, ok := m["updated_at"].(string); ok {
		c.UpdatedAt = v
	}
	if v, ok := m["deleted_at"].(string); ok {
		c.DeletedAt = &v
	}
	return nil
}

type ProductModel struct {
	ID         string  `json:"id"`
	CategoryID string  `json:"category_id"`
	Name       string  `json:"name"`
	Price      int64   `json:"price"`
	Version    int     `json:"version"`
	CreatedAt  string  `json:"created_at"`
	UpdatedAt  string  `json:"updated_at"`
	DeletedAt  *string `json:"deleted_at"`
}

func (p *ProductModel) TableName() string                 { return "products" }
func (p *ProductModel) PrimaryKey() (string, interface{}) { return "id", p.ID }
func (p *ProductModel) ToMap() map[string]interface{} {
	m := map[string]interface{}{
		"id":          p.ID,
		"category_id": p.CategoryID,
		"name":        p.Name,
		"price":       p.Price,
		"version":     p.Version,
		"created_at":  p.CreatedAt,
		"updated_at":  p.UpdatedAt,
	}
	if p.DeletedAt != nil {
		m["deleted_at"] = *p.DeletedAt
	}
	return m
}
func (p *ProductModel) FromMap(m map[string]interface{}) error {
	if v, ok := m["id"].(string); ok {
		p.ID = v
	}
	if v, ok := m["category_id"].(string); ok {
		p.CategoryID = v
	}
	if v, ok := m["name"].(string); ok {
		p.Name = v
	}
	if v, ok := m["price"].(int64); ok {
		p.Price = v
	} else if v, ok := m["price"].(int); ok {
		p.Price = int64(v)
	}
	if v, ok := m["version"].(int64); ok {
		p.Version = int(v)
	} else if v, ok := m["version"].(int); ok {
		p.Version = v
	}
	if v, ok := m["created_at"].(string); ok {
		p.CreatedAt = v
	}
	if v, ok := m["updated_at"].(string); ok {
		p.UpdatedAt = v
	}
	if v, ok := m["deleted_at"].(string); ok {
		p.DeletedAt = &v
	}
	return nil
}

// ─── TEST SETUP ─────────────────────────────────────────────────────────────

func setupModelTestDB(t *testing.T) *data.RealDBPool {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "model_test.db")
	pool, err := data.OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = pool.Close() })

	// Create test tables
	_, err = pool.Exec(`
		CREATE TABLE categories (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			deleted_at TEXT
		);
		CREATE TABLE products (
			id TEXT PRIMARY KEY,
			category_id TEXT NOT NULL REFERENCES categories(id),
			name TEXT NOT NULL,
			price BIGINT NOT NULL,
			version INT NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			deleted_at TEXT
		);
	`)
	if err != nil {
		t.Fatalf("create tables: %v", err)
	}

	return pool
}

// ─── TYPED ORM TESTS ────────────────────────────────────────────────────────

func TestTypedORMCRUDAndPagination(t *testing.T) {
	pool := setupModelTestDB(t)

	catRepo := data.NewTypedRepository(pool, func() *CategoryModel {
		return &CategoryModel{}
	}).EnableSoftDelete()

	// 1. Create
	cat := &CategoryModel{ID: "cat-1", Name: "Beverages"}
	if err := catRepo.Create(cat); err != nil {
		t.Fatalf("create category: %v", err)
	}
	if cat.CreatedAt == "" || cat.UpdatedAt == "" {
		t.Errorf("expected auto timestamps populated")
	}

	// 2. Find by ID
	found, exists, err := catRepo.Query().Find("cat-1")
	if err != nil || !exists || found.Name != "Beverages" {
		t.Fatalf("find failed: exists=%v, found=%+v, err=%v", exists, found, err)
	}

	// 3. FindBy column
	foundBy, exists, err := catRepo.Query().FindBy("name", "Beverages")
	if err != nil || !exists || foundBy.ID != "cat-1" {
		t.Fatalf("findBy failed: exists=%v, found=%+v, err=%v", exists, foundBy, err)
	}

	// 4. Update
	found.Name = "Drinks & Beverages"
	if err := catRepo.Update(found); err != nil {
		t.Fatalf("update failed: %v", err)
	}
	recheck, _, _ := catRepo.Query().Find("cat-1")
	if recheck.Name != "Drinks & Beverages" {
		t.Errorf("update did not persist: %s", recheck.Name)
	}

	// 5. Seed 10 more categories and Paginate
	for i := 2; i <= 11; i++ {
		_ = catRepo.Create(&CategoryModel{ID: fmt.Sprintf("cat-%d", i), Name: fmt.Sprintf("Category %d", i)})
	}

	page1, total, err := catRepo.Query().OrderBy("id", "ASC").Paginate(1, 5)
	if err != nil || total != 11 || len(page1) != 5 {
		t.Fatalf("paginate page 1 failed: total=%d, len=%d, err=%v", total, len(page1), err)
	}

	page2, total2, err := catRepo.Query().OrderBy("id", "ASC").Paginate(2, 5)
	if err != nil || total2 != 11 || len(page2) != 5 {
		t.Fatalf("paginate page 2 failed: total=%d, len=%d, err=%v", total2, len(page2), err)
	}

	page3, _, _ := catRepo.Query().OrderBy("id", "ASC").Paginate(3, 5)
	if len(page3) != 1 {
		t.Errorf("page 3 expected 1 item, got %d", len(page3))
	}
	t.Logf("✓ Typed ORM CRUD & Pagination verified across %d records", total)
}

func TestTypedORMPreloadEagerLoading(t *testing.T) {
	pool := setupModelTestDB(t)

	catRepo := data.NewTypedRepository(pool, func() *CategoryModel {
		return &CategoryModel{}
	})

	prodRepo := data.NewTypedRepository(pool, func() *ProductModel {
		return &ProductModel{}
	})

	// Register relationship: Category HAS_MANY Products
	catRepo.DefineRelation(data.RelationDef{
		Name:        "Products",
		Kind:        data.RelHasMany,
		ForeignKey:  "category_id",
		TargetTable: "products",
		Assign: func(parent data.Model, rows []map[string]interface{}) {
			cat := parent.(*CategoryModel)
			cat.Products = make([]*ProductModel, len(rows))
			for i, r := range rows {
				p := &ProductModel{}
				_ = p.FromMap(r)
				cat.Products[i] = p
			}
		},
	})

	// Seed 2 categories
	_ = catRepo.Create(&CategoryModel{ID: "c1", Name: "Food"})
	_ = catRepo.Create(&CategoryModel{ID: "c2", Name: "Electronics"})

	// Seed 3 products in c1, 2 products in c2
	_ = prodRepo.Create(&ProductModel{ID: "p1", CategoryID: "c1", Name: "Bread", Price: 50})
	_ = prodRepo.Create(&ProductModel{ID: "p2", CategoryID: "c1", Name: "Butter", Price: 120})
	_ = prodRepo.Create(&ProductModel{ID: "p3", CategoryID: "c1", Name: "Cheese", Price: 300})
	_ = prodRepo.Create(&ProductModel{ID: "p4", CategoryID: "c2", Name: "USB Cable", Price: 250})
	_ = prodRepo.Create(&ProductModel{ID: "p5", CategoryID: "c2", Name: "Mouse", Price: 800})

	// Query categories WITH Preload("Products") — single batch query, no N+1!
	categories, err := catRepo.Query().OrderBy("id", "ASC").Preload("Products").All()
	if err != nil {
		t.Fatalf("preload query failed: %v", err)
	}

	if len(categories) != 2 {
		t.Fatalf("expected 2 categories, got %d", len(categories))
	}
	if len(categories[0].Products) != 3 {
		t.Errorf("category c1 expected 3 preloaded products, got %d", len(categories[0].Products))
	}
	if len(categories[1].Products) != 2 {
		t.Errorf("category c2 expected 2 preloaded products, got %d", len(categories[1].Products))
	}

	t.Logf("✓ Preload correctly loaded %d and %d child products without N+1 queries",
		len(categories[0].Products), len(categories[1].Products))
}

func TestTypedORMOptimisticLocking(t *testing.T) {
	pool := setupModelTestDB(t)

	prodRepo := data.NewTypedRepository(pool, func() *ProductModel {
		return &ProductModel{}
	}).EnableOptimisticLock()

	prod := &ProductModel{ID: "p-opt", CategoryID: "c1", Name: "Tablet", Price: 15000}
	_, _ = pool.Exec(`INSERT INTO categories (id, name, created_at, updated_at) VALUES ('c1', 'Tech', '', '')`)
	if err := prodRepo.Create(prod); err != nil {
		t.Fatalf("create product: %v", err)
	}
	if prod.Version != 1 {
		t.Errorf("expected initial version=1, got %d", prod.Version)
	}

	// Concurrent reader A and B get version 1
	buyerA, _, _ := prodRepo.Query().Find("p-opt")
	buyerB, _, _ := prodRepo.Query().Find("p-opt")

	// Buyer A updates first -> succeeds, version becomes 2
	buyerA.Price = 16000
	if err := prodRepo.Update(buyerA); err != nil {
		t.Fatalf("buyer A update should succeed: %v", err)
	}
	if buyerA.Version != 2 {
		t.Errorf("expected buyer A version incremented to 2, got %d", buyerA.Version)
	}

	// Buyer B tries to update with stale version 1 -> MUST FAIL with conflict!
	buyerB.Price = 17000
	err := prodRepo.Update(buyerB)
	if err == nil {
		t.Fatalf("buyer B update with stale version should have failed with optimistic lock conflict")
	}
	t.Logf("✓ Optimistic locking prevented stale update: %v", err)
}

func TestTypedORMSoftDeleteAndTransaction(t *testing.T) {
	pool := setupModelTestDB(t)

	catRepo := data.NewTypedRepository(pool, func() *CategoryModel {
		return &CategoryModel{}
	}).EnableSoftDelete()

	_ = catRepo.Create(&CategoryModel{ID: "c-del", Name: "Discontinued"})

	// Soft delete
	if err := catRepo.SoftDelete("c-del"); err != nil {
		t.Fatalf("soft delete failed: %v", err)
	}

	// Query should NOT find it by default
	_, exists, _ := catRepo.Query().Find("c-del")
	if exists {
		t.Errorf("soft-deleted record should be hidden from normal query")
	}

	// WithTrashed SHOULD find it
	trashed, existsTrash, _ := catRepo.Query().WithTrashed().Find("c-del")
	if !existsTrash || trashed.DeletedAt == nil {
		t.Errorf("WithTrashed should reveal soft-deleted record with DeletedAt timestamp")
	}
	t.Log("✓ Soft delete hides record from normal queries and reveals with WithTrashed")

	// Transactional rollback test
	txErr := pool.Transaction(func(tx *data.RealTx) error {
		err := catRepo.CreateTx(tx, &CategoryModel{ID: "c-rollback", Name: "Will Abort"})
		if err != nil {
			return err
		}
		return fmt.Errorf("intentional abort")
	})
	if txErr == nil {
		t.Fatalf("expected error from aborted tx")
	}

	_, existsRollback, _ := catRepo.Query().Find("c-rollback")
	if existsRollback {
		t.Errorf("transactionally rolled back record should not exist in database")
	}
	t.Log("✓ Model operations within transaction rolled back completely on error")
}
