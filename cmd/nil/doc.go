package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/joysriramsarkar/nilLang/compiler/oracle"
)

// L11.2: API reference auto-generator.
//
// `nil doc` renders the compiler oracle's builtin/type tables plus the
// standard-library module catalog as Markdown (default) or JSON (--json),
// for humans and for tooling respectively:
//
//	nil doc                 short index of everything documented
//	nil doc <symbol>        Markdown reference for one symbol or std module
//	nil doc --json          the whole reference as JSON
//	nil doc <symbol> --json one entry as JSON

// docEntry is one reference item, whatever its source.
type docEntry struct {
	Name       string   `json:"name"`
	Kind       string   `json:"kind"`
	Signature  string   `json:"signature"`
	Doc        string   `json:"doc"`
	State      string   `json:"state"`
	Effects    []string `json:"effects,omitempty"`
	Parameters []string `json:"parameters,omitempty"`
	ReturnType string   `json:"return_type,omitempty"`
}

// stdModuleDoc catalogues the standard library for `nil doc`.
//
// The descriptions are short Bengali summaries; the normative documentation
// lives in the module files themselves (std/*.nil) and in std/README.md.
var stdModuleDoc = map[string]docEntry{
	"std/core":                {Kind: "MODULE", Signature: "std/core", Doc: "মূল সহায়ক: identity, clamp, Result/Optional হেলপার, pipeline।"},
	"std/math":                {Kind: "MODULE", Signature: "std/math", Doc: "গাণিতিক ফাংশন: abs/min/max/gcd/lcm/factorial/fibonacci; PI, E।"},
	"std/math/decimal":        {Kind: "MODULE", Signature: "std/math/decimal", Doc: "নির্ভুল fixed-point দশমিক (টাকার হিসাবে float ভুল নেই)।"},
	"std/money":               {Kind: "MODULE", Signature: "std/money", Doc: "মুদ্রা-সচেতন টাকা: ofMinor/ofMajor/add/sub/mulQty/format।"},
	"std/pos":                 {Kind: "MODULE", Signature: "std/pos", Doc: "POS: catalog/cart/discount/tax/checkout/বাংলা রসিদ।"},
	"std/strings":             {Kind: "MODULE", Signature: "std/strings", Doc: "স্ট্রিং ইউটিলিটি: padLeft/padRight/truncate সহ।"},
	"std/collections":         {Kind: "MODULE", Signature: "std/collections", Doc: "ফাংশনাল অ্যারে: map/filter/reduce/zip/unique/partition।"},
	"std/testing":             {Kind: "MODULE", Signature: "std/testing", Doc: "অ্যাসার্ট, মক, বেঞ্চমার্ক, গোল্ডেন।"},
	"std/io":                  {Kind: "MODULE", Signature: "std/io", Doc: "কনসোল/ফাইল/পাথ/JSON/CSV ফ্যাসাড।"},
	"std/io/path":             {Kind: "MODULE", Signature: "std/io/path", Doc: "পাথ ম্যানিপুলেশন: join/basename/dirname/ext।"},
	"std/fs":                  {Kind: "MODULE", Signature: "std/fs", Doc: "ফাইলসিস্টেম: পড়া/লেখা/তালিকা/ডিরেক্টরি।"},
	"std/process":             {Kind: "MODULE", Signature: "std/process", Doc: "প্রসেস চালানো ও exit code।"},
	"std/time":                {Kind: "MODULE", Signature: "std/time", Doc: "সময়-সুবিধা: now/sleep/today/ফরম্যাট/duration।"},
	"std/time/instant":        {Kind: "MODULE", Signature: "std/time/instant", Doc: "মুহূর্ত: তুলনা, deadline, elapsed।"},
	"std/time/duration":       {Kind: "MODULE", Signature: "std/time/duration", Doc: "সময়কাল গণিত + বাংলা formatBn।"},
	"std/time/date":           {Kind: "MODULE", Signature: "std/time/date", Doc: "তারিখ: UTC ফরম্যাট, today, isoNow।"},
	"std/net/url":             {Kind: "MODULE", Signature: "std/net/url", Doc: "URL পার্সিং, encode/decode, query তৈরি।"},
	"std/net/socket":          {Kind: "MODULE", Signature: "std/net/socket", Doc: "TCP ক্লায়েন্ট: connect/send/receive/close।"},
	"std/net/tls":             {Kind: "MODULE", Signature: "std/net/tls", Doc: "TLS ক্লায়েন্ট (সিস্টেম রুট যাচাই)।"},
	"std/net/http":            {Kind: "MODULE", Signature: "std/net/http", Doc: "HTTP ক্লায়েন্ট + ইন-প্রসেস রাউটার।"},
	"std/net/http/router":     {Kind: "MODULE", Signature: "std/net/http/router", Doc: "রাউটার: new/get/post/handle + response বিল্ডার।"},
	"std/http":                {Kind: "MODULE", Signature: "std/http", Doc: "HTTP সুবিধা-মডিউল (net/http-এর ওপর)।"},
	"std/data/json":           {Kind: "MODULE", Signature: "std/data/json", Doc: "JSON encode/decode (নেটিভ serializer)।"},
	"std/json":                {Kind: "MODULE", Signature: "std/json", Doc: "JSON সুবিধা-মডিউল।"},
	"std/data/base64":         {Kind: "MODULE", Signature: "std/data/base64", Doc: "Base64 encode/decode।"},
	"std/base64":              {Kind: "MODULE", Signature: "std/base64", Doc: "Base64 সুবিধা-মডিউল।"},
	"std/data/csv":            {Kind: "MODULE", Signature: "std/data/csv", Doc: "CSV parse/encodeRow।"},
	"std/data/migrate":        {Kind: "MODULE", Signature: "std/data/migrate", Doc: "SQLite মাইগ্রেশন রানার।"},
	"std/text/encoding":       {Kind: "MODULE", Signature: "std/text/encoding", Doc: "UTF-8 রূপান্তর, lines/words।"},
	"std/text/unicode":        {Kind: "MODULE", Signature: "std/text/unicode", Doc: "রুন-নিরাপদ লেখা-সহায়ক।"},
	"std/text/bn":             {Kind: "MODULE", Signature: "std/text/bn", Doc: "বাংলা অঙ্ক, বঙ্গাব্দ, মাস/ঋতু।"},
	"std/crypto":              {Kind: "MODULE", Signature: "std/crypto", Doc: "ক্রিপ্টো এক-জানালা: hash/HMAC/random/AES/Ed25519/base64।"},
	"std/crypto/hash":         {Kind: "MODULE", Signature: "std/crypto/hash", Doc: "SHA-256/SHA-512।"},
	"std/crypto/hmac":         {Kind: "MODULE", Signature: "std/crypto/hmac", Doc: "HMAC-SHA256 স্বাক্ষর ও verify।"},
	"std/crypto/random":       {Kind: "MODULE", Signature: "std/crypto/random", Doc: "CSPRNG bytes/hex/key32/nonce12।"},
	"std/crypto/aes":          {Kind: "MODULE", Signature: "std/crypto/aes", Doc: "AES-GCM এনক্রিপ্ট/ডিক্রিপ্ট।"},
	"std/crypto/ed25519":      {Kind: "MODULE", Signature: "std/crypto/ed25519", Doc: "Ed25519 keypair/sign/verify।"},
	"std/concurrency/task":    {Kind: "MODULE", Signature: "std/concurrency/task", Doc: "টাস্ক: spawn/spawnWith/wait/mapAsync।"},
	"std/concurrency/channel": {Kind: "MODULE", Signature: "std/concurrency/channel", Doc: "চ্যানেল + pipeline।"},
	"std/concurrency/sync":    {Kind: "MODULE", Signature: "std/concurrency/sync", Doc: "mutex ও once-গার্ড।"},
	"std/concurrency/pool":    {Kind: "MODULE", Signature: "std/concurrency/pool", Doc: "সীমিত worker pool (ক্রম-রক্ষিত)।"},
	"std/device":              {Kind: "MODULE", Signature: "std/device", Doc: "ডিভাইস FFI স্টাব (ক্যামেরা/লোকেশন/সেন্সর/বায়োমেট্রিক)।"},
	"std/device/camera":       {Kind: "MODULE", Signature: "std/device/camera", Doc: "ক্যামেরা FFI স্টাব।"},
	"std/device/location":     {Kind: "MODULE", Signature: "std/device/location", Doc: "লোকেশন FFI স্টাব।"},
	"std/device/sensors":      {Kind: "MODULE", Signature: "std/device/sensors", Doc: "সেন্সর FFI স্টাব।"},
	"std/device/biometric":    {Kind: "MODULE", Signature: "std/device/biometric", Doc: "বায়োমেট্রিক FFI স্টাব।"},
	"std/mobile":              {Kind: "MODULE", Signature: "std/mobile", Doc: "মোবাইল: lifecycle/deep-link/notify/share।"},
	"std/mobile/lifecycle":    {Kind: "MODULE", Signature: "std/mobile/lifecycle", Doc: "অ্যাপ lifecycle state machine।"},
	"std/mobile/deeplink":     {Kind: "MODULE", Signature: "std/mobile/deeplink", Doc: "Deep link parse/route।"},
	"std/mobile/notify":       {Kind: "MODULE", Signature: "std/mobile/notify", Doc: "নোটিফিকেশন সারি।"},
	"std/mobile/share":        {Kind: "MODULE", Signature: "std/mobile/share", Doc: "শেয়ার-শিট।"},
	"std/db":                  {Kind: "MODULE", Signature: "std/db", Doc: "SQL: connect/query/exec/transaction/migrate।"},
	"std/db/query_builder":    {Kind: "MODULE", Signature: "std/db/query_builder", Doc: "Fluent SQL বিল্ডার।"},
}

// docEntries merges the oracle's builtin/type tables with the std catalog,
// sorted by name so output is deterministic.
func docEntries() []docEntry {
	o := oracle.NewCompilerOracle()
	seen := make(map[string]bool)
	var out []docEntry

	for _, sym := range o.ListFunctions() {
		out = append(out, docEntry{
			Name:       sym.Name,
			Kind:       sym.Kind,
			Signature:  sym.TypeSig,
			Doc:        sym.Doc,
			State:      string(sym.State),
			Effects:    sym.Effects,
			Parameters: sym.Parameters,
			ReturnType: sym.ReturnType,
		})
		seen[sym.Name] = true
	}
	for _, name := range o.ListTypes() {
		if seen[name] {
			continue
		}
		sym, found := o.FindSymbol(name)
		entry := docEntry{Name: name, Kind: "TYPE", Signature: "type", State: "STABLE"}
		if found {
			entry.Doc = sym.Doc
			entry.State = string(sym.State)
		}
		out = append(out, entry)
		seen[name] = true
	}
	names := make([]string, 0, len(stdModuleDoc))
	for name := range stdModuleDoc {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		entry := stdModuleDoc[name]
		entry.Name = name
		entry.State = "STABLE"
		out = append(out, entry)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// docFind looks up one symbol, accepting the std/ prefix optionally and
// matching case-insensitively as a fallback.
func docFind(name string) (docEntry, bool) {
	entries := docEntries()
	for _, e := range entries {
		if e.Name == name {
			return e, true
		}
	}
	if !strings.HasPrefix(name, "std/") {
		if e, ok := docFind("std/" + name); ok {
			return e, true
		}
	}
	lowered := strings.ToLower(name)
	for _, e := range entries {
		if strings.ToLower(e.Name) == lowered {
			return e, true
		}
	}
	return docEntry{}, false
}

// docMarkdown renders one entry as Markdown.
func docMarkdown(e docEntry) string {
	var b strings.Builder
	b.WriteString("## `" + e.Name + "`\n\n")
	b.WriteString("- **Kind:** " + e.Kind + "\n")
	if e.Signature != "" {
		b.WriteString("- **Signature:** `" + e.Signature + "`\n")
	}
	if e.State != "" {
		b.WriteString("- **State:** " + e.State + "\n")
	}
	if len(e.Parameters) > 0 {
		b.WriteString("- **Parameters:** `" + strings.Join(e.Parameters, "`, `") + "`\n")
	}
	if e.ReturnType != "" {
		b.WriteString("- **Returns:** `" + e.ReturnType + "`\n")
	}
	if len(e.Effects) > 0 {
		b.WriteString("- **Effects:** " + strings.Join(e.Effects, ", ") + "\n")
	}
	if e.Doc != "" {
		b.WriteString("\n" + e.Doc + "\n")
	}
	return b.String()
}

// docIndexMarkdown renders the short index of everything documented.
func docIndexMarkdown() string {
	var b strings.Builder
	b.WriteString("# Nilang API Reference (সংক্ষিপ্ত সূচি)\n\n")
	b.WriteString("পূর্ণ বিবরণের জন্য: `nil doc <নাম>`; JSON-এর জন্য: `nil doc --json`\n\n")
	for _, e := range docEntries() {
		line := "- `" + e.Name + "` (" + e.Kind + ")"
		if e.Signature != "" && e.Signature != e.Name {
			line += " — `" + e.Signature + "`"
		}
		b.WriteString(line + "\n")
	}
	return b.String()
}

// docJSON renders entries as indented JSON.
func docJSON(entries []docEntry) string {
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return "[]"
	}
	return string(data)
}

func printDocUsage() {
	fmt.Println("📖 Nilang API Reference")
	fmt.Println("ব্যবহার: nil doc [নাম] [--json]")
	fmt.Println()
	fmt.Println("উদাহরণ:")
	fmt.Println("  nil doc              # সংক্ষিপ্ত সূচি (Markdown)")
	fmt.Println("  nil doc len          # একটি বিল্ট-ইনের বিবরণ")
	fmt.Println("  nil doc std/pos      # একটি std মডিউলের বিবরণ")
	fmt.Println("  nil doc --json       # পুরো রেফারেন্স JSON-এ")
}

func cmdDoc() {
	args := os.Args[2:]
	wantJSON := false
	var names []string
	for _, a := range args {
		switch a {
		case "--json", "-j":
			wantJSON = true
		case "--help", "-h", "help":
			printDocUsage()
			return
		default:
			names = append(names, a)
		}
	}

	if len(names) == 0 {
		if wantJSON {
			fmt.Println(docJSON(docEntries()))
		} else {
			fmt.Println(docIndexMarkdown())
		}
		return
	}
	if len(names) > 1 {
		fmt.Fprintf(os.Stderr, "❌ একটি নাম দিন: nil doc <নাম>\n")
		os.Exit(1)
	}
	entry, found := docFind(names[0])
	if !found {
		fmt.Fprintf(os.Stderr, "❌ %q পাওয়া যায়নি। `nil doc` দিয়ে সূচি দেখুন।\n", names[0])
		os.Exit(1)
	}
	if wantJSON {
		fmt.Println(docJSON([]docEntry{entry}))
	} else {
		fmt.Println(docMarkdown(entry))
	}
}
