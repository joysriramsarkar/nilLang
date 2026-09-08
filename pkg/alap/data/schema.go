package data

// POSSchema defines the canonical DDL migrations for the nilLang POS database.
// All 23 entity tables are created with proper FK constraints, indexes, and versioning.
// Apply via: runner := NewRealMigrationRunner(pool); RegisterPOSMigrations(runner); runner.Up()

// RegisterPOSMigrations registers all versioned POS schema migrations.
func RegisterPOSMigrations(runner *RealMigrationRunner) {
	// Version 1: Core identity tables
	runner.Register(Migration{
		Version: 1,
		Name:    "create_organizations_and_stores",
		UpSQL: `
CREATE TABLE IF NOT EXISTS organizations (
	id            TEXT PRIMARY KEY,
	name          TEXT NOT NULL,
	name_bn       TEXT,
	tax_reg_no    TEXT,
	phone         TEXT,
	email         TEXT,
	address       TEXT,
	currency      TEXT NOT NULL DEFAULT 'BDT',
	timezone      TEXT NOT NULL DEFAULT 'Asia/Dhaka',
	active        INTEGER NOT NULL DEFAULT 1,
	created_at    TEXT NOT NULL,
	updated_at    TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS stores (
	id            TEXT PRIMARY KEY,
	org_id        TEXT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
	name          TEXT NOT NULL,
	name_bn       TEXT,
	phone         TEXT,
	address       TEXT,
	active        INTEGER NOT NULL DEFAULT 1,
	created_at    TEXT NOT NULL,
	updated_at    TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_stores_org ON stores(org_id);
`,
		DownSQL: `DROP TABLE IF EXISTS stores; DROP TABLE IF EXISTS organizations;`,
	})

	// Version 2: Register + Shift
	runner.Register(Migration{
		Version: 2,
		Name:    "create_registers_and_shifts",
		UpSQL: `
CREATE TABLE IF NOT EXISTS registers (
	id          TEXT PRIMARY KEY,
	store_id    TEXT NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
	name        TEXT NOT NULL,
	device_id   TEXT,
	status      TEXT NOT NULL DEFAULT 'CLOSED',
	active      INTEGER NOT NULL DEFAULT 1,
	created_at  TEXT NOT NULL,
	updated_at  TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_registers_store ON registers(store_id);

CREATE TABLE IF NOT EXISTS shifts (
	id                    TEXT PRIMARY KEY,
	register_id           TEXT NOT NULL,
	cashier_id            TEXT NOT NULL,
	cashier_name          TEXT NOT NULL,
	status                TEXT NOT NULL DEFAULT 'OPEN',
	start_time            TEXT NOT NULL,
	end_time              TEXT,
	starting_cash_minor   INTEGER NOT NULL DEFAULT 0,
	cash_sales_minor      INTEGER NOT NULL DEFAULT 0,
	card_sales_minor      INTEGER NOT NULL DEFAULT 0,
	mfs_sales_minor       INTEGER NOT NULL DEFAULT 0,
	total_sales_minor     INTEGER NOT NULL DEFAULT 0,
	total_orders          INTEGER NOT NULL DEFAULT 0,
	expected_cash_minor   INTEGER NOT NULL DEFAULT 0,
	actual_cash_minor     INTEGER NOT NULL DEFAULT 0,
	cash_difference_minor INTEGER NOT NULL DEFAULT 0,
	notes                 TEXT,
	created_at            TEXT NOT NULL,
	updated_at            TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_shifts_register ON shifts(register_id);
CREATE INDEX IF NOT EXISTS idx_shifts_status   ON shifts(status);

CREATE TABLE IF NOT EXISTS cash_movements (
	id           TEXT PRIMARY KEY,
	shift_id     TEXT NOT NULL REFERENCES shifts(id) ON DELETE CASCADE,
	type         TEXT NOT NULL,
	amount_minor INTEGER NOT NULL,
	reason       TEXT NOT NULL,
	timestamp    TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_cash_movements_shift ON cash_movements(shift_id);
`,
		DownSQL: `DROP TABLE IF EXISTS cash_movements; DROP TABLE IF EXISTS shifts; DROP TABLE IF EXISTS registers;`,
	})

	// Version 3: User RBAC
	runner.Register(Migration{
		Version: 3,
		Name:    "create_users_roles_permissions",
		UpSQL: `
CREATE TABLE IF NOT EXISTS roles (
	id          TEXT PRIMARY KEY,
	name        TEXT NOT NULL UNIQUE,
	description TEXT,
	created_at  TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS permissions (
	id          TEXT PRIMARY KEY,
	name        TEXT NOT NULL UNIQUE,
	description TEXT,
	created_at  TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS role_permissions (
	role_id       TEXT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
	permission_id TEXT NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
	PRIMARY KEY (role_id, permission_id)
);

CREATE TABLE IF NOT EXISTS users (
	id          TEXT PRIMARY KEY,
	org_id      TEXT NOT NULL,
	name        TEXT NOT NULL,
	phone       TEXT,
	email       TEXT,
	pin_hash    TEXT,
	role_id     TEXT REFERENCES roles(id),
	active      INTEGER NOT NULL DEFAULT 1,
	created_at  TEXT NOT NULL,
	updated_at  TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_users_org ON users(org_id);
`,
		DownSQL: `DROP TABLE IF EXISTS users; DROP TABLE IF EXISTS role_permissions; DROP TABLE IF EXISTS permissions; DROP TABLE IF EXISTS roles;`,
	})

	// Version 4: Product catalog
	runner.Register(Migration{
		Version: 4,
		Name:    "create_product_catalog",
		UpSQL: `
CREATE TABLE IF NOT EXISTS categories (
	id          TEXT PRIMARY KEY,
	name        TEXT NOT NULL,
	name_bn     TEXT,
	icon        TEXT,
	parent_id   TEXT REFERENCES categories(id),
	active      INTEGER NOT NULL DEFAULT 1,
	created_at  TEXT NOT NULL,
	updated_at  TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS brands (
	id          TEXT PRIMARY KEY,
	name        TEXT NOT NULL,
	created_at  TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS units (
	id          TEXT PRIMARY KEY,
	name        TEXT NOT NULL UNIQUE,
	symbol      TEXT NOT NULL,
	dimension   TEXT NOT NULL,
	to_base     TEXT NOT NULL DEFAULT '1.0',
	created_at  TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS products (
	id             TEXT PRIMARY KEY,
	sku            TEXT NOT NULL UNIQUE,
	barcode        TEXT UNIQUE,
	name           TEXT NOT NULL,
	name_bn        TEXT,
	description    TEXT,
	category_id    TEXT REFERENCES categories(id),
	brand_id       TEXT REFERENCES brands(id),
	unit           TEXT NOT NULL,
	price_minor    INTEGER NOT NULL DEFAULT 0,
	cost_minor     INTEGER NOT NULL DEFAULT 0,
	stock_raw      INTEGER NOT NULL DEFAULT 0,
	low_stock_raw  INTEGER NOT NULL DEFAULT 0,
	currency       TEXT NOT NULL DEFAULT 'BDT',
	tax_rate_raw   INTEGER NOT NULL DEFAULT 0,
	active         INTEGER NOT NULL DEFAULT 1,
	version        INTEGER NOT NULL DEFAULT 0,
	created_at     TEXT NOT NULL,
	updated_at     TEXT NOT NULL,
	deleted_at     TEXT
);
CREATE INDEX IF NOT EXISTS idx_products_sku     ON products(sku);
CREATE INDEX IF NOT EXISTS idx_products_barcode ON products(barcode);
CREATE INDEX IF NOT EXISTS idx_products_category ON products(category_id);
CREATE INDEX IF NOT EXISTS idx_products_active   ON products(active);

CREATE TABLE IF NOT EXISTS product_variants (
	id          TEXT PRIMARY KEY,
	product_id  TEXT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
	sku         TEXT NOT NULL UNIQUE,
	barcode     TEXT UNIQUE,
	name        TEXT NOT NULL,
	price_minor INTEGER NOT NULL DEFAULT 0,
	stock_raw   INTEGER NOT NULL DEFAULT 0,
	active      INTEGER NOT NULL DEFAULT 1,
	created_at  TEXT NOT NULL,
	updated_at  TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_variants_product ON product_variants(product_id);
`,
		DownSQL: `DROP TABLE IF EXISTS product_variants; DROP TABLE IF EXISTS products; DROP TABLE IF EXISTS units; DROP TABLE IF EXISTS brands; DROP TABLE IF EXISTS categories;`,
	})

	// Version 5: Inventory ledger
	runner.Register(Migration{
		Version: 5,
		Name:    "create_inventory_ledger",
		UpSQL: `
CREATE TABLE IF NOT EXISTS stock_movements (
	id            TEXT PRIMARY KEY,
	product_id    TEXT NOT NULL REFERENCES products(id),
	product_name  TEXT NOT NULL,
	type          TEXT NOT NULL,
	delta_raw     INTEGER NOT NULL,
	balance_raw   INTEGER NOT NULL,
	cost_minor    INTEGER NOT NULL DEFAULT 0,
	reference     TEXT,
	timestamp     TEXT NOT NULL,
	notes         TEXT
);
CREATE INDEX IF NOT EXISTS idx_movements_product   ON stock_movements(product_id);
CREATE INDEX IF NOT EXISTS idx_movements_type      ON stock_movements(type);
CREATE INDEX IF NOT EXISTS idx_movements_timestamp ON stock_movements(timestamp);
`,
		DownSQL: `DROP TABLE IF EXISTS stock_movements;`,
	})

	// Version 6: Customer + customer ledger
	runner.Register(Migration{
		Version: 6,
		Name:    "create_customers_and_ledger",
		UpSQL: `
CREATE TABLE IF NOT EXISTS customers (
	id                     TEXT PRIMARY KEY,
	name                   TEXT NOT NULL,
	phone                  TEXT,
	email                  TEXT,
	address                TEXT,
	total_purchases_minor  INTEGER NOT NULL DEFAULT 0,
	due_balance_minor      INTEGER NOT NULL DEFAULT 0,
	credit_limit_minor     INTEGER NOT NULL DEFAULT 0,
	active                 INTEGER NOT NULL DEFAULT 1,
	created_at             TEXT NOT NULL,
	updated_at             TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_customers_phone ON customers(phone);

CREATE TABLE IF NOT EXISTS customer_ledger (
	id              TEXT PRIMARY KEY,
	customer_id     TEXT NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
	type            TEXT NOT NULL,
	amount_minor    INTEGER NOT NULL,
	balance_after   INTEGER NOT NULL,
	reference       TEXT,
	notes           TEXT,
	created_by      TEXT,
	timestamp       TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_cledger_customer ON customer_ledger(customer_id);

CREATE TABLE IF NOT EXISTS suppliers (
	id          TEXT PRIMARY KEY,
	name        TEXT NOT NULL,
	phone       TEXT,
	email       TEXT,
	address     TEXT,
	payable_minor INTEGER NOT NULL DEFAULT 0,
	active      INTEGER NOT NULL DEFAULT 1,
	created_at  TEXT NOT NULL,
	updated_at  TEXT NOT NULL
);
`,
		DownSQL: `DROP TABLE IF EXISTS suppliers; DROP TABLE IF EXISTS customer_ledger; DROP TABLE IF EXISTS customers;`,
	})

	// Version 7: Sales core
	runner.Register(Migration{
		Version: 7,
		Name:    "create_sales",
		UpSQL: `
CREATE TABLE IF NOT EXISTS sales (
	id              TEXT PRIMARY KEY,
	invoice_number  TEXT NOT NULL UNIQUE,
	register_id     TEXT NOT NULL,
	cashier_id      TEXT NOT NULL,
	customer_id     TEXT,
	shift_id        TEXT,
	subtotal_minor  INTEGER NOT NULL DEFAULT 0,
	discount_minor  INTEGER NOT NULL DEFAULT 0,
	tax_minor       INTEGER NOT NULL DEFAULT 0,
	total_minor     INTEGER NOT NULL DEFAULT 0,
	paid_minor      INTEGER NOT NULL DEFAULT 0,
	change_minor    INTEGER NOT NULL DEFAULT 0,
	status          TEXT NOT NULL DEFAULT 'COMPLETED',
	notes           TEXT,
	created_at      TEXT NOT NULL,
	updated_at      TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_sales_register   ON sales(register_id);
CREATE INDEX IF NOT EXISTS idx_sales_cashier    ON sales(cashier_id);
CREATE INDEX IF NOT EXISTS idx_sales_customer   ON sales(customer_id);
CREATE INDEX IF NOT EXISTS idx_sales_invoice    ON sales(invoice_number);
CREATE INDEX IF NOT EXISTS idx_sales_created_at ON sales(created_at);
CREATE INDEX IF NOT EXISTS idx_sales_status     ON sales(status);

CREATE TABLE IF NOT EXISTS sale_items (
	id               TEXT PRIMARY KEY,
	sale_id          TEXT NOT NULL REFERENCES sales(id) ON DELETE CASCADE,
	product_id       TEXT NOT NULL,
	name             TEXT NOT NULL,
	sku              TEXT,
	unit             TEXT,
	quantity_raw     INTEGER NOT NULL,
	unit_price_minor INTEGER NOT NULL,
	cost_price_minor INTEGER NOT NULL DEFAULT 0,
	subtotal_minor   INTEGER NOT NULL,
	discount_minor   INTEGER NOT NULL DEFAULT 0,
	tax_minor        INTEGER NOT NULL DEFAULT 0,
	profit_minor     INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_sale_items_sale    ON sale_items(sale_id);
CREATE INDEX IF NOT EXISTS idx_sale_items_product ON sale_items(product_id);

CREATE TABLE IF NOT EXISTS sale_payments (
	id            TEXT PRIMARY KEY,
	sale_id       TEXT NOT NULL REFERENCES sales(id) ON DELETE CASCADE,
	method        TEXT NOT NULL,
	amount_minor  INTEGER NOT NULL,
	reference     TEXT,
	status        TEXT NOT NULL DEFAULT 'COMPLETED',
	created_at    TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_payments_sale ON sale_payments(sale_id);
`,
		DownSQL: `DROP TABLE IF EXISTS sale_payments; DROP TABLE IF EXISTS sale_items; DROP TABLE IF EXISTS sales;`,
	})

	// Version 8: Tax + Discount + Promotion
	runner.Register(Migration{
		Version: 8,
		Name:    "create_tax_discount_promotion",
		UpSQL: `
CREATE TABLE IF NOT EXISTS taxes (
	id          TEXT PRIMARY KEY,
	name        TEXT NOT NULL,
	rate_raw    INTEGER NOT NULL,
	inclusive   INTEGER NOT NULL DEFAULT 0,
	compound    INTEGER NOT NULL DEFAULT 0,
	active      INTEGER NOT NULL DEFAULT 1,
	created_at  TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS discounts (
	id          TEXT PRIMARY KEY,
	code        TEXT,
	name        TEXT NOT NULL,
	type        TEXT NOT NULL,
	value_raw   INTEGER NOT NULL,
	min_qty     INTEGER NOT NULL DEFAULT 0,
	max_uses    INTEGER NOT NULL DEFAULT 0,
	uses        INTEGER NOT NULL DEFAULT 0,
	valid_from  TEXT,
	valid_until TEXT,
	active      INTEGER NOT NULL DEFAULT 1,
	created_at  TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_discounts_code ON discounts(code);
`,
		DownSQL: `DROP TABLE IF EXISTS discounts; DROP TABLE IF EXISTS taxes;`,
	})

	// Version 9: Purchase orders
	runner.Register(Migration{
		Version: 9,
		Name:    "create_purchases",
		UpSQL: `
CREATE TABLE IF NOT EXISTS purchases (
	id              TEXT PRIMARY KEY,
	po_number       TEXT NOT NULL UNIQUE,
	supplier_id     TEXT REFERENCES suppliers(id),
	status          TEXT NOT NULL DEFAULT 'PENDING',
	subtotal_minor  INTEGER NOT NULL DEFAULT 0,
	tax_minor       INTEGER NOT NULL DEFAULT 0,
	total_minor     INTEGER NOT NULL DEFAULT 0,
	received_at     TEXT,
	notes           TEXT,
	created_at      TEXT NOT NULL,
	updated_at      TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS purchase_items (
	id               TEXT PRIMARY KEY,
	purchase_id      TEXT NOT NULL REFERENCES purchases(id) ON DELETE CASCADE,
	product_id       TEXT NOT NULL REFERENCES products(id),
	quantity_raw     INTEGER NOT NULL,
	received_raw     INTEGER NOT NULL DEFAULT 0,
	cost_minor       INTEGER NOT NULL,
	subtotal_minor   INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_purchase_items_po ON purchase_items(purchase_id);
`,
		DownSQL: `DROP TABLE IF EXISTS purchase_items; DROP TABLE IF EXISTS purchases;`,
	})

	// Version 10: Refunds
	runner.Register(Migration{
		Version: 10,
		Name:    "create_refunds",
		UpSQL: `
CREATE TABLE IF NOT EXISTS refunds (
	id              TEXT PRIMARY KEY,
	sale_id         TEXT NOT NULL REFERENCES sales(id),
	cashier_id      TEXT NOT NULL,
	manager_id      TEXT,
	type            TEXT NOT NULL DEFAULT 'FULL',
	reason          TEXT NOT NULL,
	subtotal_minor  INTEGER NOT NULL DEFAULT 0,
	tax_minor       INTEGER NOT NULL DEFAULT 0,
	total_minor     INTEGER NOT NULL DEFAULT 0,
	status          TEXT NOT NULL DEFAULT 'COMPLETED',
	created_at      TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_refunds_sale ON refunds(sale_id);

CREATE TABLE IF NOT EXISTS refund_items (
	id               TEXT PRIMARY KEY,
	refund_id        TEXT NOT NULL REFERENCES refunds(id) ON DELETE CASCADE,
	sale_item_id     TEXT NOT NULL REFERENCES sale_items(id),
	product_id       TEXT NOT NULL,
	quantity_raw     INTEGER NOT NULL,
	unit_price_minor INTEGER NOT NULL,
	subtotal_minor   INTEGER NOT NULL
);
`,
		DownSQL: `DROP TABLE IF EXISTS refund_items; DROP TABLE IF EXISTS refunds;`,
	})

	// Version 11: Receipt + Audit + Sync
	runner.Register(Migration{
		Version: 11,
		Name:    "create_receipts_audit_sync",
		UpSQL: `
CREATE TABLE IF NOT EXISTS receipts (
	id            TEXT PRIMARY KEY,
	sale_id       TEXT NOT NULL REFERENCES sales(id) ON DELETE CASCADE,
	invoice_no    TEXT NOT NULL,
	store_name    TEXT,
	cashier       TEXT,
	customer      TEXT,
	format        TEXT NOT NULL DEFAULT 'HTML',
	content       TEXT,
	printed       INTEGER NOT NULL DEFAULT 0,
	print_error   TEXT,
	created_at    TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_receipts_sale ON receipts(sale_id);

CREATE TABLE IF NOT EXISTS audit_log (
	id           TEXT PRIMARY KEY,
	action       TEXT NOT NULL,
	entity_id    TEXT NOT NULL,
	entity_type  TEXT NOT NULL,
	actor        TEXT NOT NULL,
	request_id   TEXT,
	session_id   TEXT,
	device_id    TEXT,
	before_json  TEXT,
	after_json   TEXT,
	notes        TEXT,
	timestamp    TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_audit_entity    ON audit_log(entity_id);
CREATE INDEX IF NOT EXISTS idx_audit_actor     ON audit_log(actor);
CREATE INDEX IF NOT EXISTS idx_audit_timestamp ON audit_log(timestamp);

CREATE TABLE IF NOT EXISTS sync_operations (
	id              TEXT PRIMARY KEY,
	operation_id    TEXT NOT NULL UNIQUE,
	device_id       TEXT NOT NULL,
	entity_type     TEXT NOT NULL,
	entity_id       TEXT NOT NULL,
	version         INTEGER NOT NULL DEFAULT 0,
	payload         TEXT NOT NULL,
	status          TEXT NOT NULL DEFAULT 'PENDING',
	attempt         INTEGER NOT NULL DEFAULT 0,
	last_error      TEXT,
	retry_after     TEXT,
	created_at      TEXT NOT NULL,
	synced_at       TEXT
);
CREATE INDEX IF NOT EXISTS idx_sync_status    ON sync_operations(status);
CREATE INDEX IF NOT EXISTS idx_sync_device    ON sync_operations(device_id);
CREATE INDEX IF NOT EXISTS idx_sync_op_id     ON sync_operations(operation_id);

CREATE TABLE IF NOT EXISTS processed_operations (
	operation_id  TEXT PRIMARY KEY,
	processed_at  TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS job_records (
	id           TEXT PRIMARY KEY,
	job_type     TEXT NOT NULL,
	status       TEXT NOT NULL DEFAULT 'PENDING',
	payload      TEXT,
	attempt      INTEGER NOT NULL DEFAULT 0,
	max_attempts INTEGER NOT NULL DEFAULT 5,
	last_error   TEXT,
	retry_at     TEXT,
	created_at   TEXT NOT NULL,
	started_at   TEXT,
	finished_at  TEXT
);
CREATE INDEX IF NOT EXISTS idx_jobs_status   ON job_records(status);
CREATE INDEX IF NOT EXISTS idx_jobs_type     ON job_records(job_type);
CREATE INDEX IF NOT EXISTS idx_jobs_retry_at ON job_records(retry_at);
`,
		DownSQL: `DROP TABLE IF EXISTS job_records; DROP TABLE IF EXISTS processed_operations; DROP TABLE IF EXISTS sync_operations; DROP TABLE IF EXISTS audit_log; DROP TABLE IF EXISTS receipts;`,
	})

	// Version 12: Supplier ledger
	runner.Register(Migration{
		Version: 12,
		Name:    "create_supplier_ledger",
		UpSQL: `
CREATE TABLE IF NOT EXISTS supplier_ledger (
	id            TEXT PRIMARY KEY,
	supplier_id   TEXT NOT NULL REFERENCES suppliers(id) ON DELETE CASCADE,
	type          TEXT NOT NULL,
	amount_minor  INTEGER NOT NULL,
	balance_after INTEGER NOT NULL,
	reference     TEXT,
	notes         TEXT,
	created_by    TEXT,
	timestamp     TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_sledger_supplier ON supplier_ledger(supplier_id);
CREATE INDEX IF NOT EXISTS idx_sledger_timestamp ON supplier_ledger(timestamp);
`,
		DownSQL: `DROP TABLE IF EXISTS supplier_ledger;`,
	})
}
