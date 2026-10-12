// Command studio serves the Nilang Studio visual UI builder stub: a web page
// with the component catalog (Dashboard/Table/Form/Card/Button) plus a JSON
// API for tooling to consume.
//
// This is a scaffold (L11.6): the catalog is real data, drag-and-drop code
// generation is future work. It intentionally depends only on the standard
// library so `go run ./tools/studio` always works.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
)

// Component describes one visual builder block.
type Component struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Props       []string `json:"props"`
	Example     string   `json:"example"`
}

// Catalog is the served component list. It mirrors the five component types
// the UI renderer understands today.
func Catalog() []Component {
	return []Component{
		{
			Name:        "Dashboard",
			Description: "মূল ড্যাশবোর্ড ধারক: শিরোনাম, মেট্রিক ও সন্তান।",
			Props:       []string{"title", "metrics", "children"},
			Example:     `Dashboard(title: "দোকান") { Card(title: "আজকের বিক্রি") }`,
		},
		{
			Name:        "Table",
			Description: "সারি/কলাম টেবিল: headers + rows।",
			Props:       []string{"headers", "rows"},
			Example:     `Table(headers: ["নাম", "দাম"], rows: [["চা", 20]])`,
		},
		{
			Name:        "Form",
			Description: "ইনপুট ফর্ম: fields + onSubmit।",
			Props:       []string{"fields", "onSubmit"},
			Example:     `Form(fields: ["নাম", "ফোন"])`,
		},
		{
			Name:        "Card",
			Description: "শিরোনামসহ তথ্য-কার্ড।",
			Props:       []string{"title", "body"},
			Example:     `Card(title: "মোট", body: "৳85.05")`,
		},
		{
			Name:        "Button",
			Description: "ক্রিয়া-বোতাম: label + onClick।",
			Props:       []string{"label", "onClick"},
			Example:     `Button(label: "বিক্রি", onClick: "checkout")`,
		},
	}
}

const indexPage = `<!DOCTYPE html>
<html lang="bn">
<head><meta charset="utf-8"><title>নীলাং স্টুডিও (stub)</title></head>
<body>
<h1>নীলাং স্টুডিও — component catalog</h1>
<p>Visual UI builder stub: catalog নিচে, JSON <a href="/api/components">/api/components</a>-এ।</p>
<ul id="catalog"></ul>
<script>
fetch('/api/components').then(r => r.json()).then(items => {
  const ul = document.getElementById('catalog');
  for (const c of items) {
    const li = document.createElement('li');
    li.innerHTML = '<b>' + c.name + '</b> — ' + c.description +
      '<br><code>' + c.example + '</code>';
    ul.appendChild(li);
  }
});
</script>
</body>
</html>`

// NewMux wires the studio routes; split out for tests.
func NewMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, indexPage)
	})
	mux.HandleFunc("/api/components", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(Catalog())
	})
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		fmt.Fprint(w, `{"ok":true}`)
	})
	return mux
}

func main() {
	addr := flag.String("addr", "127.0.0.1:0", "listen address (host:port)")
	flag.Parse()
	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "studio: listen %s: %v\n", *addr, err)
		os.Exit(1)
	}
	fmt.Printf("নীলাং স্টুডিও (stub): http://%s\n", ln.Addr())
	if err := http.Serve(ln, NewMux()); err != nil {
		fmt.Fprintf(os.Stderr, "studio: serve: %v\n", err)
		os.Exit(1)
	}
}
