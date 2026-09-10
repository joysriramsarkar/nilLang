package stdlib

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	_ "modernc.org/sqlite"

	"github.com/joysriramsarkar/nilLang/compiler/object"
)

var (
	dbPoolMu sync.Mutex
	dbPool   = make(map[string]*sql.DB)
)

func getDB(dsn string) (*sql.DB, error) {
	if dsn == "" {
		dsn = ":memory:"
	}
	dbPoolMu.Lock()
	defer dbPoolMu.Unlock()
	if db, ok := dbPool[dsn]; ok {
		return db, nil
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db %q: %w", dsn, err)
	}
	dbPool[dsn] = db
	return db, nil
}

// CallHost implements the host-backed part of std. It deliberately returns
// ordinary Nilang objects so the evaluator and VM can share the same contract.
func CallHost(name string, args []object.Object) (object.Object, error) {
	switch name {
	case NativeTimeSleep:
		if len(args) != 1 {
			return nil, fmt.Errorf("std.time.sleep expects 1 argument (milliseconds)")
		}
		ms, ok := args[0].(*object.Integer)
		if !ok || ms.Value < 0 {
			return nil, fmt.Errorf("std.time.sleep expects non-negative INTEGER milliseconds")
		}
		time.Sleep(time.Duration(ms.Value) * time.Millisecond)
		return &object.Null{}, nil

	case NativeDBSQLQuery:
		return dbQuery(args)

	case NativeDBSQLExec:
		return dbExec(args)
	case NativeJSONEncode:
		if len(args) != 1 {
			return nil, fmt.Errorf("std.json.encode expects 1 argument")
		}
		v, err := toGo(args[0])
		if err != nil {
			return nil, err
		}
		b, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("json encode: %w", err)
		}
		return &object.String{Value: string(b)}, nil

	case NativeJSONDecode:
		if len(args) != 1 {
			return nil, fmt.Errorf("std.json.decode expects 1 argument")
		}
		s, ok := args[0].(*object.String)
		if !ok {
			return nil, fmt.Errorf("std.json.decode expects STRING")
		}
		var v any
		if err := json.Unmarshal([]byte(s.Value), &v); err != nil {
			return nil, fmt.Errorf("json decode: %w", err)
		}
		return fromGo(v), nil

	case NativeBase64Enc:
		s, err := oneString(args, NativeBase64Enc)
		if err != nil {
			return nil, err
		}
		return &object.String{Value: base64.StdEncoding.EncodeToString([]byte(s))}, nil

	case NativeBase64Dec:
		s, err := oneString(args, NativeBase64Dec)
		if err != nil {
			return nil, err
		}
		b, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			return nil, fmt.Errorf("base64 decode: %w", err)
		}
		return &object.String{Value: string(b)}, nil

	case NativeSHA256:
		s, err := oneString(args, NativeSHA256)
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256([]byte(s))
		return &object.String{Value: hex.EncodeToString(sum[:])}, nil

	case NativeSHA512:
		s, err := oneString(args, NativeSHA512)
		if err != nil {
			return nil, err
		}
		sum := sha512.Sum512([]byte(s))
		return &object.String{Value: hex.EncodeToString(sum[:])}, nil

	case NativeHMACSHA256:
		if len(args) != 2 {
			return nil, fmt.Errorf("std.crypto.hmac_sha256 expects key and message")
		}
		key, ok1 := args[0].(*object.String)
		msg, ok2 := args[1].(*object.String)
		if !ok1 || !ok2 {
			return nil, fmt.Errorf("std.crypto.hmac_sha256 expects STRING arguments")
		}
		h := hmac.New(sha256.New, []byte(key.Value))
		_, _ = h.Write([]byte(msg.Value))
		return &object.String{Value: hex.EncodeToString(h.Sum(nil))}, nil

	case NativeRandom:
		if len(args) != 1 {
			return nil, fmt.Errorf("std.crypto.random_bytes expects count")
		}
		n, ok := args[0].(*object.Integer)
		if !ok || n.Value < 0 || n.Value > 1<<24 {
			return nil, fmt.Errorf("random byte count must be an integer in [0, 16777216]")
		}
		buf := make([]byte, n.Value)
		if _, err := io.ReadFull(rand.Reader, buf); err != nil {
			return nil, err
		}
		return &object.String{Value: base64.StdEncoding.EncodeToString(buf)}, nil

	case NativeHTTPGet, NativeHTTPPost:
		return httpRequest(name, args)
	default:
		return nil, fmt.Errorf("unknown std host function: %s", name)
	}
}

func oneString(args []object.Object, name string) (string, error) {
	if len(args) != 1 {
		return "", fmt.Errorf("%s expects 1 argument", name)
	}
	s, ok := args[0].(*object.String)
	if !ok {
		return "", fmt.Errorf("%s expects STRING", name)
	}
	return s.Value, nil
}

func httpRequest(name string, args []object.Object) (object.Object, error) {
	if (name == NativeHTTPGet && len(args) != 1) || (name == NativeHTTPPost && len(args) != 2) {
		return nil, fmt.Errorf("%s: invalid argument count", name)
	}
	urlObj, ok := args[0].(*object.String)
	if !ok {
		return nil, fmt.Errorf("http URL must be STRING")
	}
	method := http.MethodGet
	var body io.Reader
	if name == NativeHTTPPost {
		method = http.MethodPost
		b, ok := args[1].(*object.String)
		if !ok {
			return nil, fmt.Errorf("http POST body must be STRING")
		}
		body = bytes.NewBufferString(b.Value)
	}
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest(method, urlObj.Value, body)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, fmt.Errorf("http read: %w", err)
	}
	return MakeHash(map[string]object.Object{
		"status": &object.Integer{Value: int64(resp.StatusCode)},
		"body":   &object.String{Value: string(data)},
	}), nil
}

func MakeHash(values map[string]object.Object) *object.Hash {
	pairs := make(map[object.HashKey]object.HashPair, len(values))
	for k, v := range values {
		key := &object.String{Value: k}
		pairs[key.HashKey()] = object.HashPair{Key: key, Value: v}
	}
	return &object.Hash{Pairs: pairs}
}

func toGo(v object.Object) (any, error) {
	switch x := v.(type) {
	case *object.Null:
		return nil, nil
	case *object.Boolean:
		return x.Value, nil
	case *object.Integer:
		return x.Value, nil
	case *object.Float:
		return x.Value, nil
	case *object.String:
		return x.Value, nil
	case *object.Array:
		out := make([]any, len(x.Elements))
		for i, e := range x.Elements {
			g, err := toGo(e)
			if err != nil {
				return nil, err
			}
			out[i] = g
		}
		return out, nil
	case *object.Hash:
		out := make(map[string]any, len(x.Pairs))
		for _, p := range x.Pairs {
			k, ok := p.Key.(*object.String)
			if !ok {
				return nil, fmt.Errorf("json object key must be STRING")
			}
			g, err := toGo(p.Value)
			if err != nil {
				return nil, err
			}
			out[k.Value] = g
		}
		return out, nil
	default:
		return nil, fmt.Errorf("value of type %s is not JSON-serializable", v.Type())
	}
}

func fromGo(v any) object.Object {
	switch x := v.(type) {
	case nil:
		return &object.Null{}
	case bool:
		return &object.Boolean{Value: x}
	case float64:
		return &object.Float{Value: x}
	case string:
		return &object.String{Value: x}
	case []any:
		out := make([]object.Object, len(x))
		for i, e := range x {
			out[i] = fromGo(e)
		}
		return &object.Array{Elements: out}
	case map[string]any:
		out := make(map[string]object.Object, len(x))
		for k, e := range x {
			out[k] = fromGo(e)
		}
		return MakeHash(out)
	default:
		return &object.Null{}
	}
}

func dbQuery(args []object.Object) (object.Object, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("std.db.query expects (dsn, query, [params])")
	}
	dsnObj, ok := args[0].(*object.String)
	if !ok {
		return nil, fmt.Errorf("std.db.query expects dsn to be STRING")
	}
	queryObj, ok := args[1].(*object.String)
	if !ok {
		return nil, fmt.Errorf("std.db.query expects query to be STRING")
	}
	var params []any
	if len(args) >= 3 {
		if arr, ok := args[2].(*object.Array); ok {
			for _, elem := range arr.Elements {
				g, err := toGo(elem)
				if err != nil {
					return nil, err
				}
				params = append(params, g)
			}
		}
	}
	db, err := getDB(dsnObj.Value)
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(queryObj.Value, params...)
	if err != nil {
		return nil, fmt.Errorf("db query error: %w", err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	var resultRows []object.Object
	for rows.Next() {
		colValues := make([]any, len(cols))
		colPointers := make([]any, len(cols))
		for i := range colValues {
			colPointers[i] = &colValues[i]
		}
		if err := rows.Scan(colPointers...); err != nil {
			return nil, fmt.Errorf("row scan error: %w", err)
		}
		rowMap := make(map[string]object.Object, len(cols))
		for i, col := range cols {
			val := colValues[i]
			if b, ok := val.([]byte); ok {
				rowMap[col] = &object.String{Value: string(b)}
			} else {
				rowMap[col] = fromGo(val)
			}
		}
		resultRows = append(resultRows, MakeHash(rowMap))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &object.Array{Elements: resultRows}, nil
}

func dbExec(args []object.Object) (object.Object, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("std.db.exec expects (dsn, query, [params])")
	}
	dsnObj, ok := args[0].(*object.String)
	if !ok {
		return nil, fmt.Errorf("std.db.exec expects dsn to be STRING")
	}
	queryObj, ok := args[1].(*object.String)
	if !ok {
		return nil, fmt.Errorf("std.db.exec expects query to be STRING")
	}
	var params []any
	if len(args) >= 3 {
		if arr, ok := args[2].(*object.Array); ok {
			for _, elem := range arr.Elements {
				g, err := toGo(elem)
				if err != nil {
					return nil, err
				}
				params = append(params, g)
			}
		}
	}
	db, err := getDB(dsnObj.Value)
	if err != nil {
		return nil, err
	}
	res, err := db.Exec(queryObj.Value, params...)
	if err != nil {
		return nil, fmt.Errorf("db exec error: %w", err)
	}
	affected, _ := res.RowsAffected()
	return &object.Integer{Value: affected}, nil
}
