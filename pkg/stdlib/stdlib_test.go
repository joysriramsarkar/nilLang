package stdlib_test

import (
	"net"
	"testing"
	"time"

	"github.com/joysriramsarkar/nilLang/compiler/object"
	"github.com/joysriramsarkar/nilLang/pkg/stdlib"
)

func TestBundledStandardModules(t *testing.T) {
	for _, name := range []string{
		"std/core", "std/math", "std/math/decimal", "std/money",
		"std/strings", "std/collections", "std/testing", "std/io",
		"std/io/path", "std/fs", "std/data/csv", "std/data/json",
		"std/data/base64", "std/data/migrate",
		"std/crypto", "std/crypto/hash", "std/crypto/hmac", "std/crypto/random",
		"std/crypto/aes", "std/crypto/ed25519",
		"std/net/http", "std/net/http/router", "std/net/url",
		"std/net/socket", "std/net/tls",
		"std/http", "std/json", "std/base64", "std/time",
		"std/time/instant", "std/time/duration", "std/time/date",
		"std/text/encoding", "std/text/unicode", "std/text/bn",
		"std/db", "std/db/query_builder", "std/pos",
		"std/concurrency/task", "std/concurrency/channel",
		"std/concurrency/sync", "std/concurrency/pool",
		"std/device", "std/device/camera", "std/device/location",
		"std/device/sensors", "std/device/biometric",
		"std/mobile", "std/mobile/lifecycle", "std/mobile/deeplink",
		"std/mobile/notify", "std/mobile/share",
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

func TestHostChannelSelect(t *testing.T) {
	empty := object.NewChannel(2)
	full := object.NewChannel(2)
	full.Values <- &object.String{Value: "ready"}
	out, err := stdlib.CallHost(stdlib.NativeChannelPick, []object.Object{empty, full})
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	picked, ok := out.(*object.Hash)
	if !ok {
		t.Fatalf("expected hash, got %T", out)
	}
	idx := picked.Pairs[(&object.String{Value: "index"}).HashKey()].Value.(*object.Integer).Value
	val := picked.Pairs[(&object.String{Value: "value"}).HashKey()].Value.(*object.String).Value
	if idx != 1 || val != "ready" {
		t.Fatalf("expected {1, ready}, got {%d, %q}", idx, val)
	}
	if _, err := stdlib.CallHost(stdlib.NativeChannelPick, []object.Object{&object.Integer{Value: 1}}); err == nil {
		t.Fatal("expected CHANNEL type rejection")
	}
}

func TestHostAESGCM(t *testing.T) {
	randKey, err := stdlib.CallHost(stdlib.NativeRandom, []object.Object{&object.Integer{Value: 32}})
	if err != nil {
		t.Fatalf("random key: %v", err)
	}
	randNonce, err := stdlib.CallHost(stdlib.NativeRandom, []object.Object{&object.Integer{Value: 12}})
	if err != nil {
		t.Fatalf("random nonce: %v", err)
	}
	plain := &object.String{Value: "গোপন বার্তা"}
	sealed, err := stdlib.CallHost(stdlib.NativeAESEncrypt, []object.Object{randKey, randNonce, plain})
	if err != nil {
		t.Fatalf("aes encrypt: %v", err)
	}
	opened, err := stdlib.CallHost(stdlib.NativeAESDecrypt, []object.Object{randKey, randNonce, sealed})
	if err != nil {
		t.Fatalf("aes decrypt: %v", err)
	}
	if opened.(*object.String).Value != plain.Value {
		t.Fatalf("aes round trip mismatch: %q", opened.(*object.String).Value)
	}
	// Wrong key must fail authentication, not return garbage.
	wrongKey, _ := stdlib.CallHost(stdlib.NativeRandom, []object.Object{&object.Integer{Value: 32}})
	if _, err := stdlib.CallHost(stdlib.NativeAESDecrypt, []object.Object{wrongKey, randNonce, sealed}); err == nil {
		t.Fatal("expected authentication failure with wrong key")
	}
	// Bad key length must be rejected.
	short := &object.String{Value: "eA=="}
	if _, err := stdlib.CallHost(stdlib.NativeAESEncrypt, []object.Object{short, randNonce, plain}); err == nil {
		t.Fatal("expected key-length rejection")
	}
}

func TestHostEd25519(t *testing.T) {
	kp, err := stdlib.CallHost(stdlib.NativeEdKeypair, nil)
	if err != nil {
		t.Fatalf("keypair: %v", err)
	}
	hash, ok := kp.(*object.Hash)
	if !ok {
		t.Fatalf("expected hash, got %T", kp)
	}
	pubKey := hash.Pairs[(&object.String{Value: "public_key"}).HashKey()].Value
	privKey := hash.Pairs[(&object.String{Value: "private_key"}).HashKey()].Value
	msg := &object.String{Value: "চুক্তি v1"}
	sig, err := stdlib.CallHost(stdlib.NativeEdSign, []object.Object{privKey, msg})
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	okObj, err := stdlib.CallHost(stdlib.NativeEdVerify, []object.Object{pubKey, msg, sig})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !okObj.(*object.Boolean).Value {
		t.Fatal("expected valid signature to verify")
	}
	tampered := &object.String{Value: "চুক্তি v2"}
	okObj, err = stdlib.CallHost(stdlib.NativeEdVerify, []object.Object{pubKey, tampered, sig})
	if err != nil {
		t.Fatalf("verify tampered: %v", err)
	}
	if okObj.(*object.Boolean).Value {
		t.Fatal("expected tampered message to fail verification")
	}
}

func TestHostTimeFormat(t *testing.T) {
	out, err := stdlib.CallHost(stdlib.NativeTimeFormat, []object.Object{
		&object.Integer{Value: 0},
		&object.String{Value: "YYYY-MM-DD hh:mm:ss"},
	})
	if err != nil {
		t.Fatalf("format: %v", err)
	}
	if out.(*object.String).Value != "1970-01-01 00:00:00" {
		t.Fatalf("unexpected epoch format: %q", out.(*object.String).Value)
	}
	iso, err := stdlib.CallHost(stdlib.NativeTimeNowISO, nil)
	if err != nil {
		t.Fatalf("now_iso: %v", err)
	}
	if len(iso.(*object.String).Value) < 10 {
		t.Fatalf("unexpected iso value: %q", iso.(*object.String).Value)
	}
}

func TestHostURLCoding(t *testing.T) {
	enc, err := stdlib.CallHost(stdlib.NativeURLEncode, []object.Object{&object.String{Value: "শূন্য থেকে নীলাং"}})
	if err != nil {
		t.Fatalf("url encode: %v", err)
	}
	dec, err := stdlib.CallHost(stdlib.NativeURLDecode, []object.Object{enc})
	if err != nil {
		t.Fatalf("url decode: %v", err)
	}
	if dec.(*object.String).Value != "শূন্য থেকে নীলাং" {
		t.Fatalf("url round trip mismatch: %q", dec.(*object.String).Value)
	}
}

func TestHostHashKeys(t *testing.T) {
	h := stdlib.MakeHash(map[string]object.Object{
		"b": &object.Integer{Value: 2},
		"a": &object.Integer{Value: 1},
		"c": &object.Integer{Value: 3},
	})
	out, err := stdlib.CallHost(stdlib.NativeHashKeys, []object.Object{h})
	if err != nil {
		t.Fatalf("hash_keys: %v", err)
	}
	arr := out.(*object.Array)
	if len(arr.Elements) != 3 {
		t.Fatalf("expected 3 keys, got %v", out)
	}
	got := arr.Elements[0].Inspect() + arr.Elements[1].Inspect() + arr.Elements[2].Inspect()
	if got != "abc" {
		t.Fatalf("expected sorted keys abc, got %q", got)
	}
	if _, err := stdlib.CallHost(stdlib.NativeHashKeys, []object.Object{&object.Integer{Value: 1}}); err == nil {
		t.Fatal("expected HASH type rejection")
	}
}

func TestHostTCPAndDevice(t *testing.T) {
	// Loopback echo server: accept one connection, echo one message back.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	port := int64(ln.Addr().(*net.TCPAddr).Port)
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 64)
		_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
		n, err := conn.Read(buf)
		if err != nil || n == 0 {
			return
		}
		_, _ = conn.Write([]byte("echo:" + string(buf[:n])))
	}()

	host := &object.String{Value: "127.0.0.1"}
	handle, err := stdlib.CallHost(stdlib.NativeTCPConnect, []object.Object{host, &object.Integer{Value: port}})
	if err != nil {
		t.Fatalf("tcp connect: %v", err)
	}
	if _, err := stdlib.CallHost(stdlib.NativeTCPSend, []object.Object{handle, &object.String{Value: "ping"}}); err != nil {
		t.Fatalf("tcp send: %v", err)
	}
	reply, err := stdlib.CallHost(stdlib.NativeTCPReceive, []object.Object{handle, &object.Integer{Value: 64}})
	if err != nil {
		t.Fatalf("tcp receive: %v", err)
	}
	if reply.(*object.String).Value != "echo:ping" {
		t.Fatalf("unexpected echo: %q", reply.(*object.String).Value)
	}
	if _, err := stdlib.CallHost(stdlib.NativeTCPClose, []object.Object{handle}); err != nil {
		t.Fatalf("tcp close: %v", err)
	}
	<-done

	// Connection refused must be a clean error, not a hang.
	badPort := &object.Integer{Value: 1}
	if _, err := stdlib.CallHost(stdlib.NativeTCPConnect, []object.Object{host, badPort}); err == nil {
		t.Fatal("expected connection-refused error")
	}

	info, err := stdlib.CallHost(stdlib.NativeDeviceInfo, nil)
	if err != nil {
		t.Fatalf("device info: %v", err)
	}
	if _, ok := info.(*object.Hash); !ok {
		t.Fatalf("expected hash, got %T", info)
	}
	note, err := stdlib.CallHost(stdlib.NativeMobileNotify, []object.Object{&object.String{Value: "স্বাগতম"}})
	if err != nil {
		t.Fatalf("notify: %v", err)
	}
	queued := note.(*object.Hash).Pairs[(&object.String{Value: "queued"}).HashKey()].Value
	if !queued.(*object.Boolean).Value {
		t.Fatal("expected queued=true")
	}
}
