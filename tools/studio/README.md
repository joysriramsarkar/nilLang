# নীলাং স্টুডিও (`tools/studio`) — L11.6 scaffold

Visual UI builder-এর stub: component catalog-এর ওয়েব পেজ সার্ভ করে।

## চালানো

```sh
go run ./tools/studio --addr 127.0.0.1:8080
# নীলাং স্টুডিও (stub): http://127.0.0.1:8080
```

- `/` — catalog পেজ (Dashboard/Table/Form/Card/Button)।
- `/api/components` — catalog JSON (টুলিং-এর জন্য)।
- `/api/health` — `{"ok":true}`।

## পরীক্ষা

```sh
go test ./tools/studio/
```

## ভবিষ্যৎ

Drag-and-drop থেকে `.nil` কোড-জেনারেশন এই scaffold-এর পরের ধাপ; catalog-ই
সেই জেনারেটরের চুক্তি (contract) — নাম/props/example স্থিতিশীল থাকবে।
