package stdlib_test

import (
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/object"
	"github.com/joysriramsarkar/nilLang/pkg/stdlib"
)

func TestBundledStandardModules(t *testing.T) {
	for _, name := range []string{
		"std/core", "std/math", "std/math/decimal", "std/money",
		"std/strings", "std/collections", "std/testing", "std/io",
		"std/fs", "std/data/csv", "std/data/json", "std/data/base64",
		"std/crypto", "std/crypto/hash", "std/crypto/hmac", "std/crypto/random",
		"std/net/http", "std/net/http/router", "std/net/url",
		"std/http", "std/json", "std/base64", "std/time",
		"std/db", "std/db/query_builder",
		"std/concurrency/task", "std/concurrency/channel",
	} {
		if !stdlib.Exists(name) {
			t.Fatalf("missing bundled module %q", name)
		}
	}
}

func TestUnknownStandardModule(t *testing.T) {
	if _, err := stdlib.Read("std/does-not-exist"); err == nil {
		t.Fatal("expected unknown standard module to fail")
	}
}

func TestHostJSON(t *testing.T) {
	// Encode Hash
	hashObj := stdlib.MakeHash(map[string]object.Object{
		"name": &object.String{Value: "Nilang"},
		"ver":  &object.Integer{Value: 1},
	})
	encoded, err := stdlib.CallHost(stdlib.NativeJSONEncode, []object.Object{hashObj})
	if err != nil {
		t.Fatalf("JSON encode failed: %v", err)
	}
	strObj, ok := encoded.(*object.String)
	if !ok || len(strObj.Value) == 0 {
		t.Fatalf("expected JSON string, got %T: %+v", encoded, encoded)
	}

	// Decode JSON
	decoded, err := stdlib.CallHost(stdlib.NativeJSONDecode, []object.Object{strObj})
	if err != nil {
		t.Fatalf("JSON decode failed: %v", err)
	}
	decHash, ok := decoded.(*object.Hash)
	if !ok {
		t.Fatalf("expected decoded Hash, got %T", decoded)
	}
	nameKey := (&object.String{Value: "name"}).HashKey()
	if pair, exists := decHash.Pairs[nameKey]; !exists || pair.Value.(*object.String).Value != "Nilang" {
		t.Fatalf("expected name=Nilang in decoded hash, got: %+v", decHash)
	}
}

func TestHostCryptoAndBase64(t *testing.T) {
	inputStr := &object.String{Value: "Hello, Nilang!"}

	// Base64 Encode & Decode
	b64, err := stdlib.CallHost(stdlib.NativeBase64Enc, []object.Object{inputStr})
	if err != nil {
		t.Fatalf("Base64 encode error: %v", err)
	}
	b64Dec, err := stdlib.CallHost(stdlib.NativeBase64Dec, []object.Object{b64})
	if err != nil {
		t.Fatalf("Base64 decode error: %v", err)
	}
	if b64Dec.(*object.String).Value != inputStr.Value {
		t.Fatalf("expected %q, got %q", inputStr.Value, b64Dec.(*object.String).Value)
	}

	// SHA256
	sha, err := stdlib.CallHost(stdlib.NativeSHA256, []object.Object{inputStr})
	if err != nil {
		t.Fatalf("SHA256 error: %v", err)
	}
	if len(sha.(*object.String).Value) != 64 {
		t.Fatalf("expected 64-char hex string, got %s", sha.(*object.String).Value)
	}

	// HMAC-SHA256
	keyStr := &object.String{Value: "secret-key"}
	hmacRes, err := stdlib.CallHost(stdlib.NativeHMACSHA256, []object.Object{keyStr, inputStr})
	if err != nil {
		t.Fatalf("HMAC error: %v", err)
	}
	if len(hmacRes.(*object.String).Value) != 64 {
		t.Fatalf("expected 64-char hex string, got %s", hmacRes.(*object.String).Value)
	}

	// Random bytes
	randCount := &object.Integer{Value: 16}
	randBytes, err := stdlib.CallHost(stdlib.NativeRandom, []object.Object{randCount})
	if err != nil {
		t.Fatalf("Random error: %v", err)
	}
	if len(randBytes.(*object.String).Value) == 0 {
		t.Fatalf("expected non-empty base64 random bytes")
	}
}

func TestHostDBAndSleep(t *testing.T) {
	// Sleep
	sleepMs := &object.Integer{Value: 5}
	if _, err := stdlib.CallHost(stdlib.NativeTimeSleep, []object.Object{sleepMs}); err != nil {
		t.Fatalf("Sleep error: %v", err)
	}

	// SQLite In-Memory DB
	dsn := &object.String{Value: "file:testdb?mode=memory&cache=shared"}
	createSQL := &object.String{Value: "CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT, price INTEGER);"}
	if _, err := stdlib.CallHost(stdlib.NativeDBSQLExec, []object.Object{dsn, createSQL}); err != nil {
		t.Fatalf("DB create table failed: %v", err)
	}

	insertSQL := &object.String{Value: "INSERT INTO items (name, price) VALUES (?, ?);"}
	insertParams := &object.Array{Elements: []object.Object{
		&object.String{Value: "Nilang Book"},
		&object.Integer{Value: 450},
	}}
	res, err := stdlib.CallHost(stdlib.NativeDBSQLExec, []object.Object{dsn, insertSQL, insertParams})
	if err != nil {
		t.Fatalf("DB insert failed: %v", err)
	}
	if affected, ok := res.(*object.Integer); !ok || affected.Value != 1 {
		t.Fatalf("expected 1 row affected, got %v", res)
	}

	querySQL := &object.String{Value: "SELECT id, name, price FROM items WHERE price >= ?;"}
	queryParams := &object.Array{Elements: []object.Object{&object.Integer{Value: 400}}}
	qRes, err := stdlib.CallHost(stdlib.NativeDBSQLQuery, []object.Object{dsn, querySQL, queryParams})
	if err != nil {
		t.Fatalf("DB query failed: %v", err)
	}
	rows, ok := qRes.(*object.Array)
	if !ok || len(rows.Elements) != 1 {
		t.Fatalf("expected 1 row returned, got %v", qRes)
	}
	firstRow, ok := rows.Elements[0].(*object.Hash)
	if !ok {
		t.Fatalf("expected row to be Hash, got %T", rows.Elements[0])
	}
	nameKey := (&object.String{Value: "name"}).HashKey()
	if p, exists := firstRow.Pairs[nameKey]; !exists || p.Value.(*object.String).Value != "Nilang Book" {
		t.Fatalf("expected name='Nilang Book', got %+v", firstRow)
	}
}
