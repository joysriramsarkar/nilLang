package tests

import (
	"fmt"
	"net"
	"testing"
)

// L10 conformance, part 2: concurrency, database, Bengali text, time,
// device/mobile stubs, the test module itself, and the POS package.

func TestStdlibConcurrency(t *testing.T) {
	checkBothEngines(t, `
import "std/concurrency/task" as tasks;
import "std/concurrency/channel" as ch;
let doubled = tasks.mapAsync([1, 2, 3], fn(x) { return x * 2; });
let waited = tasks.waitAll([tasks.spawnValue(7), tasks.spawnValue(8)]);
let c = ch.create(8);
ch.sendAll(c, ["a", "b", "c"]);
let got = ch.receiveAll(c, 3);
let pipe = ch.pipeline(c, 2, fn(v) { return v + "!"; });
ch.sendAll(c, ["x", "y"]);
let pf = pipe["done"];
let pd = tasks.wait(pf);
let prow = ch.receiveAll(pipe["output"], 2);
let result = str(doubled[0]) + str(doubled[2]) + str(waited[1]) + got[0] + got[2] + str(pd) + prow[0] + prow[1];
`, "result", "268ac2x!y!")
}

func TestStdlibMutexAndPool(t *testing.T) {
	checkBothEngines(t, `
import "std/concurrency/sync" as sync;
import "std/concurrency/pool" as pool;
let m = sync.newMutex();
let guarded = sync.withLock(m, fn() { return 40 + 2; });
let squares = pool.map([1, 2, 3, 4], fn(x) { return x * x; }, 2);
let n = pool.runEach(["a", "b", "c"], fn(s) { return s; }, 2);
let once = sync.newOnce();
let first = sync.doOnce(once, fn() { return "এক"; });
let result = str(guarded) + "|" + str(squares[0]) + str(squares[1]) + str(squares[2]) + str(squares[3]) + "|" + str(n) + "|" + first["value"];
`, "result", "42|14916|3|এক")
}

func TestStdlibQueryBuilder(t *testing.T) {
	checkBothEngines(t, `
import "std/db/query_builder" as qb;
let q = qb.new("orders");
qb.select(q, ["id", "customer"]);
qb.where(q, "amount", ">=", 100);
qb.whereIn(q, "status", ["paid", "ready"]);
qb.order(q, "id", false);
qb.limit(q, 5);
qb.offset(q, 10);
let sql = qb.buildSql(q);
let csql = qb.countSql(q);
let q2 = qb.new("t");
qb.distinct(q2, true);
qb.select(q2, ["k"]);
let dsql = qb.buildSql(q2);
let result = sql + " || " + csql + " || " + dsql;
`, "result", "SELECT id, customer FROM orders WHERE amount >= ? AND status IN (?, ?) ORDER BY id ASC LIMIT 5 OFFSET 10 || SELECT COUNT(*) AS total FROM orders WHERE amount >= ? AND status IN (?, ?) || SELECT DISTINCT k FROM t")
}

func TestStdlibDBMigrate(t *testing.T) {
	dsn := "file:l10migrate?mode=memory&cache=shared"
	checkBothEngines(t, fmt.Sprintf(`
import "std/db" as db;
let conn = db.connect(%q);
db.exec(conn, "DROP TABLE IF EXISTS users;", []);
db.exec(conn, "DROP TABLE IF EXISTS _nilang_migrations;", []);
let ms = [db.migrateEntry(1, "create-users", "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT);"), db.migrateEntry(2, "seed", "INSERT INTO users (name) VALUES ('রহিম');")];
let applied = db.migrateUp(conn, ms);
let again = db.migrateUp(conn, ms);
let one = db.queryOne(conn, "SELECT name FROM users;", []);
db.insertMap(conn, "users", {"name": "করিম"});
let cnt = db.countRows(db.table("users"), conn);
let status = db.migrateStatus(conn, ms);
let result = str(len(applied)) + str(len(again)) + one["name"] + str(cnt) + status;
`, dsn), "result", "20রহিম2applied=2 pending=0")
}
func TestStdlibDBTransaction(t *testing.T) {
	dsn := "file:l10tx?mode=memory&cache=shared"
	checkBothEngines(t, fmt.Sprintf(`
import "std/db" as db;
import "std/core" as core;
let conn = db.connect(%q);
db.exec(conn, "DROP TABLE IF EXISTS items;", []);
db.exec(conn, "CREATE TABLE items (id INTEGER PRIMARY KEY, v INTEGER);", []);
let r1 = db.transaction(conn, fn() {
  db.exec(conn, "INSERT INTO items (v) VALUES (1);", []);
  return core.ok("committed");
});
let rows = db.query(conn, "SELECT v FROM items;", []);
let r2 = db.transaction(conn, fn() {
  db.exec(conn, "INSERT INTO items (v) VALUES (2);", []);
  return core.err("rolled back");
});
let rows2 = db.query(conn, "SELECT v FROM items;", []);
let result = unwrap(r1) + str(len(rows)) + str(isErr(r2)) + str(len(rows2));
`, dsn), "result", "committed1true1")
}

func TestStdlibTextBn(t *testing.T) {
	checkBothEngines(t, `
import "std/text/bn" as bn;
let digits = bn.toBnDigits("2026");
let back = bn.toLatinDigits("২০২৬");
let norm = bn.normalizeSpaces("  শূন্য   থেকে  নীলাং  ");
let year = bn.banglaYear(2026, 10, 12);
let yearApr = bn.banglaYear(2026, 3, 1);
let yearText = bn.banglaYearText(2026, 10, 12);
let month = bn.monthNameBn(10);
let season = bn.seasonNameBn(6);
let result = digits + "|" + back + "|" + norm + "|" + str(year) + str(yearApr) + "|" + yearText + "|" + month + "|" + season;
`, "result", "২০২৬|2026|শূন্য থেকে নীলাং|14331432|১৪৩৩ বঙ্গাব্দ|অক্টোবর|বর্ষা")
}

func TestStdlibTextUnicode(t *testing.T) {
	checkBothEngines(t, `
import "std/text/unicode" as u;
import "std/text/encoding" as e;
let rev = u.reverseText("abc");
let bn = u.reverseText("অআ");
let wc = e.wordCount("শূন্য থেকে নীলাং");
let w = e.words("  a  b ");
let result = rev + "|" + bn + "|" + str(wc) + "|" + str(len(w)) + w[0] + w[1] + "|" + u.firstChar("nil") + u.lastChar("nil") + "|" + u.withoutFirst("nil") + u.withoutLast("nil");
`, "result", "cba|আঅ|3|2ab|nl|ilni")
}

func TestStdlibTime(t *testing.T) {
	checkBothEngines(t, `
import "std/time" as time;
import "std/time/duration" as dur;
let epoch = time.format(0, "YYYY-MM-DD hh:mm:ss");
let d = time.formatSeconds(86400, "YYYY-MM-DD");
let fb = time.formatDurationBn(90);
let fd = time.formatDurationBn(90000);
let h = time.hours(2);
let w = time.weeks(1);
let before = time.isBefore(1, 2);
let dl = time.deadlineIn(100000);
let expiredFlag = time.isExpired(1);
let result = epoch + "|" + d + "|" + fb + "|" + fd + "|" + str(h) + str(w) + str(before) + str(expiredFlag) + str(time.minutes(3)) + str(time.days(2));
`, "result", "1970-01-01 00:00:00|1970-01-02|1 মিনিট|1 দিন|7200604800truetrue180172800")
}

func TestStdlibDeviceMobile(t *testing.T) {
	checkBothEngines(t, `
import "std/device" as device;
import "std/mobile" as mobile;
let info = device.info();
let cam = device.capturePhoto(null);
let loc = device.currentLocation();
let bio = device.authenticate("unlock");
let app = mobile.new("আমার অ্যাপ");
mobile.onResume(app);
let fg = mobile.isForeground(app);
mobile.onPause(app);
let st = mobile.appState(app);
let route = mobile.linkRoute("nilang://shop/product/42?ref=home");
let arg = mobile.parseLink("nilang://shop/product/42?ref=home")["path"];
let note = mobile.notify("স্বাগতম!");
let shared = mobile.shareText("হ্যালো");
let result = str(info["available"]) + str(isErr(cam)) + str(isErr(loc)) + str(isErr(bio)) + str(fg) + st + route + arg + str(note["queued"]) + str(shared["shared"]);
`, "result", "falsetruetruetruetruepausedproduct/product/42truetrue")
}

func TestStdlibTestingModule(t *testing.T) {
	checkBothEngines(t, `
import "std/testing" as t;
t.assertEqual(2 + 2, 4, "যোগ");
t.assertOk(Ok(1), null);
t.assertErr(Err("x"), null);
t.assertSome(Some(3), null);
t.assertLen([1, 2], 2, null);
t.assertContains("নীলাং ভাষা", "ভাষা", null);
let m = t.mockFn(fn(x) { return x * 2; });
let r = m["fn"](21);
let calls = t.mockCalls(m);
let bench = t.benchmark("sum", 5, fn() { return 1 + 1; });
let store = t.goldenStore();
t.assertGolden("greet", "হ্যালো", store);
t.assertGolden("greet", "হ্যালো", store);
let result = str(r) + str(calls[0]) + str(t.mockCallCount(m)) + str(bench["iters"]) + bench["name"];
`, "result", "422115sum")
}

func TestStdlibPOS(t *testing.T) {
	checkBothEngines(t, `
import "std/pos" as pos;
import "std/money" as money;
let catalog = pos.newCatalog();
pos.addItem(catalog, "চা", 20, "BDT");
pos.addItem(catalog, "কফি", 50, "BDT");
let cart = pos.newCart("BDT");
unwrap(pos.addToCart(catalog, cart, "চা", 2));
unwrap(pos.addToCart(catalog, cart, "কফি", 1));
let bad = pos.addToCart(catalog, cart, "লস্যি", 1);
let receipt = pos.checkout(cart, 10, 5);
let total = money.format(receipt["total"], "৳");
let slip = pos.formatReceipt(cart, receipt, "৳");
let result = str(pos.cartCount(cart)) + "|" + total + "|" + str(isErr(bad)) + "|" + str(contains(slip, "মোট:"));
`, "result", "3|৳85.05|true|true")
}

func TestStdlibSocketEcho(t *testing.T) {
	// Loopback echo server driven from Go; the .nil side is an ordinary
	// socket client. Both engines run the same client program.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 2; i++ {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			buf := make([]byte, 64)
			n, _ := conn.Read(buf)
			_, _ = conn.Write([]byte("echo:" + string(buf[:n])))
			conn.Close()
		}
	}()
	entry := fmt.Sprintf(`
import "std/net/socket" as socket;
let conn = socket.connect("127.0.0.1", %d);
socket.sendAll(conn, "ping");
let reply = socket.receive(conn, 64);
socket.close(conn);
let result = reply;
`, port)
	checkBothEngines(t, entry, "result", "echo:ping")
	<-done
}

func TestStdlibChannelSelect(t *testing.T) {
	checkBothEngines(t, `
import "std/concurrency/channel" as ch;
let c1 = ch.create(4);
let c2 = ch.create(4);
ch.sendValue(c2, "দ্বিতীয়");
let picked = ch.selectRecv([c1, c2]);
ch.sendValue(c1, "প্রথম");
let one = ch.selectRecv([c1]);
ch.sendValue(c2, "আবার");
let two = ch.selectRecv([c1, c2]);
let tooMany = ch.selectRecv([c1, c2, c1, c2]);
let result = str(picked["index"]) + picked["value"] + str(one["index"]) + one["value"] + str(two["index"]) + two["value"] + str(isErr(tooMany));
`, "result", "1দ্বিতীয়0প্রথম1আবারtrue")
}

func TestStdlibPlaceholderDialect(t *testing.T) {
	checkBothEngines(t, `
import "std/db/query_builder" as qb;
let q = qb.new("orders");
qb.select(q, ["id"]);
qb.where(q, "amount", ">=", 100);
qb.where(q, "status", "=", "paid");
let lite = qb.buildSqlFor(q, "sqlite");
let pg = qb.buildSqlFor(q, "postgres");
let plain = qb.placeholder("SELECT 1", "postgres");
let result = lite + " || " + pg + " || " + plain;
`, "result", "SELECT id FROM orders WHERE amount >= ? AND status = ? || SELECT id FROM orders WHERE amount >= $1 AND status = $2 || SELECT 1")
}
