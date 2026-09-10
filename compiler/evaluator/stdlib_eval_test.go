package evaluator

import (
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/lexer"
	"github.com/joysriramsarkar/nilLang/compiler/object"
	"github.com/joysriramsarkar/nilLang/compiler/parser"
)

func evalSource(input string) object.Object {
	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	env := object.NewEnvironment()
	return Eval(program, env)
}

func TestStdlibDecimalAndMoney(t *testing.T) {
	input := `
	import "std/math/decimal" as decimal;
	import "std/money" as money;

	let d1 = decimal.new(1250, 2); // 12.50
	let d2 = decimal.new(750, 2);  // 7.50
	let sum = decimal.add(d1, d2);
	let sumStr = decimal.toString(sum);

	let m1 = money.ofMinor(1500, "BDT"); // 15.00 BDT
	let m2 = money.ofMinor(500, "BDT");  // 5.00 BDT
	let mSum = money.add(m1, m2);
	let mFormatted = money.format(unwrap(mSum), "৳");

	[sumStr, mFormatted]
	`
	obj := evalSource(input)
	if isError(obj) {
		t.Fatalf("eval error: %s", obj.Inspect())
	}
	arr, ok := obj.(*object.Array)
	if !ok || len(arr.Elements) != 2 {
		t.Fatalf("expected array of 2 elements, got %v", obj)
	}
	if arr.Elements[0].Inspect() != "20.00" {
		t.Errorf("expected decimal sum '20.00', got %q", arr.Elements[0].Inspect())
	}
	if arr.Elements[1].Inspect() != "৳20.00" {
		t.Errorf("expected money formatted '৳20.00', got %q", arr.Elements[1].Inspect())
	}
}

func TestStdlibJSONAndCrypto(t *testing.T) {
	input := `
	import "std/json" as json;
	import "std/crypto" as crypto;

	let data = {"language": "Nilang", "version": 1};
	let encoded = json.encode(data);
	let decoded = json.decode(encoded);

	let hashVal = crypto.sha256("test-data");
	let hmacVal = crypto.hmacSha256("my-secret", "payload");

	[decoded["language"], len(hashVal), len(hmacVal)]
	`
	obj := evalSource(input)
	if isError(obj) {
		t.Fatalf("eval error: %s", obj.Inspect())
	}
	arr, ok := obj.(*object.Array)
	if !ok || len(arr.Elements) != 3 {
		t.Fatalf("expected array of 3 elements, got %v", obj)
	}
	if arr.Elements[0].Inspect() != "Nilang" {
		t.Errorf("expected decoded language 'Nilang', got %q", arr.Elements[0].Inspect())
	}
	if arr.Elements[1].Inspect() != "64" {
		t.Errorf("expected sha256 len 64, got %q", arr.Elements[1].Inspect())
	}
	if arr.Elements[2].Inspect() != "64" {
		t.Errorf("expected hmac len 64, got %q", arr.Elements[2].Inspect())
	}
}

func TestStdlibQueryBuilderAndDB(t *testing.T) {
	input := `
	import "std/db" as db;
	import "std/db/query_builder" as qb;

	let q = qb.new("orders");
	qb.select(q, ["id", "customer", "amount"]);
	qb.where(q, "amount", ">=", 100);
	qb.order(q, "id", false);
	qb.limit(q, 5);
	let sql = qb.buildSql(q);

	let conn = db.connect("file:qbtest?mode=memory&cache=shared");
	db.exec(conn, "CREATE TABLE orders (id INTEGER PRIMARY KEY, customer TEXT, amount INTEGER);", []);
	db.exec(conn, "INSERT INTO orders (customer, amount) VALUES (?, ?);", ["Rahim", 250]);
	let rows = db.query(conn, "SELECT customer, amount FROM orders WHERE amount >= ?;", [100]);

	[sql, len(rows), rows[0]["customer"]]
	`
	obj := evalSource(input)
	if isError(obj) {
		t.Fatalf("eval error: %s", obj.Inspect())
	}
	arr, ok := obj.(*object.Array)
	if !ok || len(arr.Elements) != 3 {
		t.Fatalf("expected array of 3 elements, got %v", obj)
	}
	expectedSQL := "SELECT id, customer, amount FROM orders WHERE amount >= ? ORDER BY id ASC LIMIT 5"
	if arr.Elements[0].Inspect() != expectedSQL {
		t.Errorf("expected SQL %q, got %q", expectedSQL, arr.Elements[0].Inspect())
	}
	if arr.Elements[1].Inspect() != "1" {
		t.Errorf("expected 1 row, got %q", arr.Elements[1].Inspect())
	}
	if arr.Elements[2].Inspect() != "Rahim" {
		t.Errorf("expected customer 'Rahim', got %q", arr.Elements[2].Inspect())
	}
}

func TestStdlibHTTPRouter(t *testing.T) {
	input := `
	import "std/net/http" as http;

	let router = http.newRouter();
	http.routerGet(router, "/api/ping", fn(req) {
		return http.responseText(200, "pong");
	});
	http.routerPost(router, "/api/echo", fn(req) {
		return http.responseJson(201, {"echo": req["body"]});
	});

	let req1 = {"method": "GET", "path": "/api/ping", "headers": {}, "body": ""};
	let resp1 = http.routerHandle(router, req1);

	let req404 = {"method": "GET", "path": "/unknown", "headers": {}, "body": ""};
	let resp404 = http.routerHandle(router, req404);

	[resp1["status"], resp1["body"], resp404["status"]]
	`
	obj := evalSource(input)
	if isError(obj) {
		t.Fatalf("eval error: %s", obj.Inspect())
	}
	arr, ok := obj.(*object.Array)
	if !ok || len(arr.Elements) != 3 {
		t.Fatalf("expected array of 3 elements, got %v", obj)
	}
	if arr.Elements[0].Inspect() != "200" {
		t.Errorf("expected 200, got %q", arr.Elements[0].Inspect())
	}
	if arr.Elements[1].Inspect() != "pong" {
		t.Errorf("expected 'pong', got %q", arr.Elements[1].Inspect())
	}
	if arr.Elements[2].Inspect() != "404" {
		t.Errorf("expected 404, got %q", arr.Elements[2].Inspect())
	}
}

func TestStdlibCore(t *testing.T) {
	input := `
	import "std/core" as core;

	let rOk = core.ok(42);
	let isOkVal = isOk(rOk);
	let clamped = core.clamp(150, 0, 100);
	let errVal = core.error("E404", "Not found");

	[isOkVal, clamped, errVal["code"]]
	`
	obj := evalSource(input)
	if isError(obj) {
		t.Fatalf("eval error: %s", obj.Inspect())
	}
	arr, ok := obj.(*object.Array)
	if !ok || len(arr.Elements) != 3 {
		t.Fatalf("expected array of 3 elements, got %v", obj)
	}
	if arr.Elements[0].Inspect() != "true" {
		t.Errorf("expected true, got %q", arr.Elements[0].Inspect())
	}
	if arr.Elements[1].Inspect() != "100" {
		t.Errorf("expected 100, got %q", arr.Elements[1].Inspect())
	}
	if arr.Elements[2].Inspect() != "E404" {
		t.Errorf("expected E404, got %q", arr.Elements[2].Inspect())
	}
}
