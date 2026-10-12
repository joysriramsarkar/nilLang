package stdlib

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/tls"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"

	"github.com/joysriramsarkar/nilLang/compiler/object"
)

var (
	dbPoolMu sync.Mutex
	dbPool   = make(map[string]*sql.DB)

	tcpMu     sync.Mutex
	tcpNext   = 1
	tcpConns  = make(map[int]net.Conn)
	tcpDeadln = 30 * time.Second
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

	case NativeAESEncrypt:
		return aesGCMEncrypt(args)

	case NativeAESDecrypt:
		return aesGCMDecrypt(args)

	case NativeEdKeypair:
		return ed25519Keypair(args)

	case NativeEdSign:
		return ed25519Sign(args)

	case NativeEdVerify:
		return ed25519Verify(args)

	case NativeTimeNowISO:
		if len(args) != 0 {
			return nil, fmt.Errorf("std.time.now_iso expects no arguments")
		}
		return &object.String{Value: time.Now().UTC().Format(time.RFC3339)}, nil

	case NativeTimeFormat:
		return timeFormat(args)

	case NativeURLEncode:
		s, err := oneString(args, NativeURLEncode)
		if err != nil {
			return nil, err
		}
		return &object.String{Value: url.QueryEscape(s)}, nil

	case NativeURLDecode:
		s, err := oneString(args, NativeURLDecode)
		if err != nil {
			return nil, err
		}
		decoded, err := url.QueryUnescape(s)
		if err != nil {
			return nil, fmt.Errorf("url decode: %w", err)
		}
		return &object.String{Value: decoded}, nil

	case NativeTCPConnect:
		return tcpConnect(args)

	case NativeTCPSend:
		return tcpSend(args)

	case NativeTCPReceive:
		return tcpReceive(args)

	case NativeTCPClose:
		return tcpClose(args)

	case NativeTLSConnect:
		return tlsConnect(args)

	case NativeChannelPick:
		return channelPick(args)

	case NativeHashKeys:
		return hashKeys(args)

	case NativeDeviceInfo:
		if len(args) != 0 {
			return nil, fmt.Errorf("std.device.info expects no arguments")
		}
		return MakeHash(map[string]object.Object{
			"os":         &object.String{Value: runtime.GOOS},
			"arch":       &object.String{Value: runtime.GOARCH},
			"available":  &object.Boolean{Value: false},
			"backend":    &object.String{Value: "ffi-stub"},
			"model":      &object.String{Value: "generic"},
			"go_version": &object.String{Value: runtime.Version()},
		}), nil

	case NativeMobileNotify:
		s, err := oneString(args, NativeMobileNotify)
		if err != nil {
			return nil, err
		}
		return MakeHash(map[string]object.Object{
			"queued":  &object.Boolean{Value: true},
			"message": &object.String{Value: s},
		}), nil

	case NativeAudioPlay:
		return audioPlay(args)

	default:
		return nil, fmt.Errorf("unknown std host function: %s", name)
	}
}

// audioPlay hands an audio source to the current OS's default media player.
//
// Nilang ships no MP3 decoder of its own and none would build for every Onuron
// target, so this is the portable option: download http(s) sources to a temp
// file, then ask the OS to open the file with its associated player. That is
// what "open with" does on every desktop and mobile OS the toolchain targets,
// so the app can play real audio everywhere instead of on Windows only.
func audioPlay(args []object.Object) (object.Object, error) {
	src, err := oneString(args, NativeAudioPlay)
	if err != nil {
		return nil, err
	}
	path := src
	if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
		p, err := downloadTempAudio(src)
		if err != nil {
			return nil, err
		}
		path = p
	}
	if err := openWithDefault(path); err != nil {
		return nil, err
	}
	return &object.String{Value: path}, nil
}

func downloadTempAudio(url string) (string, error) {
	client := &http.Client{Timeout: 10 * time.Minute}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("audio download: %w", err)
	}
	req.Header.Set("User-Agent", "nilang/1.0 (nilLang native music player)")
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("audio download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("audio download: HTTP %d", resp.StatusCode)
	}
	f, err := os.CreateTemp("", "nilaudio-*.mp3")
	if err != nil {
		return "", fmt.Errorf("audio temp: %w", err)
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		return "", fmt.Errorf("audio write: %w", err)
	}
	return f.Name(), nil
}

// openWithDefault launches the OS-associated opening for a local file. Every
// target the nilLang toolchain builds for has an equivalent (rundll32 /
// xdg-open / open), so the same code compiles and runs everywhere.
func openWithDefault(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", path)
	case "darwin":
		cmd = exec.Command("open", path)
	default: // linux and anything else
		cmd = exec.Command("xdg-open", path)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("cannot open %q with the default player: %w", path, err)
	}
	return nil
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

// aesGCMKey decodes a base64 AES key and checks its length.
func aesGCMKey(keyB64 string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(keyB64)
	if err != nil {
		return nil, fmt.Errorf("aes key: invalid base64: %w", err)
	}
	if len(key) != 16 && len(key) != 24 && len(key) != 32 {
		return nil, fmt.Errorf("aes key must decode to 16, 24 or 32 bytes, got %d", len(key))
	}
	return key, nil
}

// aesGCMNonce decodes a base64 12-byte GCM nonce.
func aesGCMNonce(nonceB64 string) ([]byte, error) {
	nonce, err := base64.StdEncoding.DecodeString(nonceB64)
	if err != nil {
		return nil, fmt.Errorf("aes nonce: invalid base64: %w", err)
	}
	if len(nonce) != 12 {
		return nil, fmt.Errorf("aes nonce must decode to 12 bytes, got %d", len(nonce))
	}
	return nonce, nil
}

// aesGCMEncrypt implements std.crypto.aes_gcm_encrypt(keyB64, nonceB64, plaintext).
func aesGCMEncrypt(args []object.Object) (object.Object, error) {
	if len(args) != 3 {
		return nil, fmt.Errorf("std.crypto.aes_gcm_encrypt expects (key, nonce, plaintext)")
	}
	keyStr, ok1 := args[0].(*object.String)
	nonceStr, ok2 := args[1].(*object.String)
	plainStr, ok3 := args[2].(*object.String)
	if !ok1 || !ok2 || !ok3 {
		return nil, fmt.Errorf("std.crypto.aes_gcm_encrypt expects STRING arguments")
	}
	key, err := aesGCMKey(keyStr.Value)
	if err != nil {
		return nil, err
	}
	nonce, err := aesGCMNonce(nonceStr.Value)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("aes gcm: %w", err)
	}
	sealed := gcm.Seal(nil, nonce, []byte(plainStr.Value), nil)
	return &object.String{Value: base64.StdEncoding.EncodeToString(sealed)}, nil
}

// aesGCMDecrypt implements std.crypto.aes_gcm_decrypt(keyB64, nonceB64, ciphertextB64).
func aesGCMDecrypt(args []object.Object) (object.Object, error) {
	if len(args) != 3 {
		return nil, fmt.Errorf("std.crypto.aes_gcm_decrypt expects (key, nonce, ciphertext)")
	}
	keyStr, ok1 := args[0].(*object.String)
	nonceStr, ok2 := args[1].(*object.String)
	cipherStr, ok3 := args[2].(*object.String)
	if !ok1 || !ok2 || !ok3 {
		return nil, fmt.Errorf("std.crypto.aes_gcm_decrypt expects STRING arguments")
	}
	key, err := aesGCMKey(keyStr.Value)
	if err != nil {
		return nil, err
	}
	nonce, err := aesGCMNonce(nonceStr.Value)
	if err != nil {
		return nil, err
	}
	sealed, err := base64.StdEncoding.DecodeString(cipherStr.Value)
	if err != nil {
		return nil, fmt.Errorf("aes ciphertext: invalid base64: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("aes gcm: %w", err)
	}
	plain, err := gcm.Open(nil, nonce, sealed, nil)
	if err != nil {
		return nil, fmt.Errorf("aes decrypt: authentication failed: %w", err)
	}
	return &object.String{Value: string(plain)}, nil
}

// ed25519Keypair implements std.crypto.ed25519_keypair() returning base64 keys.
func ed25519Keypair(args []object.Object) (object.Object, error) {
	if len(args) != 0 {
		return nil, fmt.Errorf("std.crypto.ed25519_keypair expects no arguments")
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("ed25519 keypair: %w", err)
	}
	return MakeHash(map[string]object.Object{
		"public_key":  &object.String{Value: base64.StdEncoding.EncodeToString(pub)},
		"private_key": &object.String{Value: base64.StdEncoding.EncodeToString(priv)},
	}), nil
}

// ed25519Sign implements std.crypto.ed25519_sign(privateKeyB64, message).
func ed25519Sign(args []object.Object) (object.Object, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("std.crypto.ed25519_sign expects (private_key, message)")
	}
	privStr, ok1 := args[0].(*object.String)
	msgStr, ok2 := args[1].(*object.String)
	if !ok1 || !ok2 {
		return nil, fmt.Errorf("std.crypto.ed25519_sign expects STRING arguments")
	}
	privBytes, err := base64.StdEncoding.DecodeString(privStr.Value)
	if err != nil {
		return nil, fmt.Errorf("ed25519 private key: invalid base64: %w", err)
	}
	if len(privBytes) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("ed25519 private key must decode to %d bytes, got %d",
			ed25519.PrivateKeySize, len(privBytes))
	}
	sig := ed25519.Sign(ed25519.PrivateKey(privBytes), []byte(msgStr.Value))
	return &object.String{Value: base64.StdEncoding.EncodeToString(sig)}, nil
}

// ed25519Verify implements std.crypto.ed25519_verify(publicKeyB64, message, sigB64).
func ed25519Verify(args []object.Object) (object.Object, error) {
	if len(args) != 3 {
		return nil, fmt.Errorf("std.crypto.ed25519_verify expects (public_key, message, signature)")
	}
	pubStr, ok1 := args[0].(*object.String)
	msgStr, ok2 := args[1].(*object.String)
	sigStr, ok3 := args[2].(*object.String)
	if !ok1 || !ok2 || !ok3 {
		return nil, fmt.Errorf("std.crypto.ed25519_verify expects STRING arguments")
	}
	pubBytes, err := base64.StdEncoding.DecodeString(pubStr.Value)
	if err != nil {
		return nil, fmt.Errorf("ed25519 public key: invalid base64: %w", err)
	}
	sigBytes, err := base64.StdEncoding.DecodeString(sigStr.Value)
	if err != nil {
		return nil, fmt.Errorf("ed25519 signature: invalid base64: %w", err)
	}
	if len(pubBytes) != ed25519.PublicKeySize || len(sigBytes) != ed25519.SignatureSize {
		return &object.Boolean{Value: false}, nil
	}
	return &object.Boolean{Value: ed25519.Verify(ed25519.PublicKey(pubBytes), []byte(msgStr.Value), sigBytes)}, nil
}

// timeFormat implements std.time.format(millis, pattern) with the tokens
// YYYY (year), MM (month), DD (day), hh (hour 24h), mm (minute), ss (second),
// evaluated in UTC so both engines and every machine agree.
func timeFormat(args []object.Object) (object.Object, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("std.time.format expects (millis, pattern)")
	}
	ms, ok1 := args[0].(*object.Integer)
	pattern, ok2 := args[1].(*object.String)
	if !ok1 || !ok2 {
		return nil, fmt.Errorf("std.time.format expects (INTEGER millis, STRING pattern)")
	}
	tm := time.UnixMilli(ms.Value).UTC()
	out := pattern.Value
	out = strings.ReplaceAll(out, "YYYY", fmt.Sprintf("%04d", tm.Year()))
	out = strings.ReplaceAll(out, "MM", fmt.Sprintf("%02d", int(tm.Month())))
	out = strings.ReplaceAll(out, "DD", fmt.Sprintf("%02d", tm.Day()))
	out = strings.ReplaceAll(out, "hh", fmt.Sprintf("%02d", tm.Hour()))
	out = strings.ReplaceAll(out, "mm", fmt.Sprintf("%02d", tm.Minute()))
	out = strings.ReplaceAll(out, "ss", fmt.Sprintf("%02d", tm.Second()))
	return &object.String{Value: out}, nil
}

// hashKeys implements std.data.hash_keys(hash) returning the keys as an
// array, sorted by (type, printed value) so every run and every engine
// agrees on the order.
func hashKeys(args []object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("std.data.hash_keys expects (hash)")
	}
	h, ok := args[0].(*object.Hash)
	if !ok {
		return nil, fmt.Errorf("std.data.hash_keys expects HASH, got %s", args[0].Type())
	}
	type keyed struct {
		sortKey string
		obj     object.Object
	}
	listed := make([]keyed, 0, len(h.Pairs))
	for _, pair := range h.Pairs {
		listed = append(listed, keyed{
			sortKey: string(pair.Key.Type()) + "\x00" + pair.Key.Inspect(),
			obj:     pair.Key,
		})
	}
	sort.Slice(listed, func(i, j int) bool { return listed[i].sortKey < listed[j].sortKey })
	elems := make([]object.Object, 0, len(listed))
	for _, k := range listed {
		elems = append(elems, k.obj)
	}
	return &object.Array{Elements: elems}, nil
}

// channelPick implements std.sync.select_recv(channels...) returning
// {"index": i, "value": v} for whichever channel delivers first, using
// reflect.Select so it truly waits on all of them at once.
func channelPick(args []object.Object) (object.Object, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("std.sync.select_recv expects at least 1 channel")
	}
	cases := make([]reflect.SelectCase, 0, len(args))
	for _, arg := range args {
		ch, ok := arg.(*object.Channel)
		if !ok {
			return nil, fmt.Errorf("std.sync.select_recv expects CHANNEL arguments, got %s", arg.Type())
		}
		cases = append(cases, reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(ch.Values),
		})
	}
	chosen, value, _ := reflect.Select(cases)
	var received object.Object = &object.Null{}
	if value.IsValid() {
		if obj, ok := value.Interface().(object.Object); ok && obj != nil {
			received = obj
		}
	}
	return MakeHash(map[string]object.Object{
		"index": &object.Integer{Value: int64(chosen)},
		"value": received,
	}), nil
}

func tcpLookup(idObj object.Object) (net.Conn, int, error) {
	idInt, ok := idObj.(*object.Integer)
	if !ok {
		return nil, 0, fmt.Errorf("tcp handle must be INTEGER")
	}
	id := int(idInt.Value)
	tcpMu.Lock()
	defer tcpMu.Unlock()
	conn, ok := tcpConns[id]
	if !ok {
		return nil, 0, fmt.Errorf("unknown tcp handle %d (already closed?)", id)
	}
	return conn, id, nil
}

// tcpConnect implements std.net.tcp_connect(host, port) returning a handle.
func tcpConnect(args []object.Object) (object.Object, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("std.net.tcp_connect expects (host, port)")
	}
	host, ok1 := args[0].(*object.String)
	port, ok2 := args[1].(*object.Integer)
	if !ok1 || !ok2 {
		return nil, fmt.Errorf("std.net.tcp_connect expects (STRING host, INTEGER port)")
	}
	if port.Value < 1 || port.Value > 65535 {
		return nil, fmt.Errorf("tcp port out of range: %d", port.Value)
	}
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host.Value, strconv.Itoa(int(port.Value))), tcpDeadln)
	if err != nil {
		return nil, fmt.Errorf("tcp connect: %w", err)
	}
	_ = conn.SetDeadline(time.Now().Add(tcpDeadln))
	tcpMu.Lock()
	id := tcpNext
	tcpNext++
	tcpConns[id] = conn
	tcpMu.Unlock()
	return &object.Integer{Value: int64(id)}, nil
}

// tcpSend implements std.net.tcp_send(handle, data) returning bytes written.
func tcpSend(args []object.Object) (object.Object, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("std.net.tcp_send expects (handle, data)")
	}
	conn, _, err := tcpLookup(args[0])
	if err != nil {
		return nil, err
	}
	data, ok := args[1].(*object.String)
	if !ok {
		return nil, fmt.Errorf("std.net.tcp_send data must be STRING")
	}
	_ = conn.SetDeadline(time.Now().Add(tcpDeadln))
	n, err := conn.Write([]byte(data.Value))
	if err != nil {
		return nil, fmt.Errorf("tcp send: %w", err)
	}
	return &object.Integer{Value: int64(n)}, nil
}

// tlsConnect implements std.net.tls_connect(host, port) with system roots.
// The TLS connection reuses the TCP handle table, so send/receive/close are
// shared: a TLS handle works with std.net.tcp_send and friends.
func tlsConnect(args []object.Object) (object.Object, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("std.net.tls_connect expects (host, port)")
	}
	host, ok1 := args[0].(*object.String)
	port, ok2 := args[1].(*object.Integer)
	if !ok1 || !ok2 {
		return nil, fmt.Errorf("std.net.tls_connect expects (STRING host, INTEGER port)")
	}
	if port.Value < 1 || port.Value > 65535 {
		return nil, fmt.Errorf("tls port out of range: %d", port.Value)
	}
	dialer := &net.Dialer{Timeout: tcpDeadln}
	conn, err := tls.DialWithDialer(dialer, "tcp",
		net.JoinHostPort(host.Value, strconv.Itoa(int(port.Value))),
		&tls.Config{ServerName: host.Value})
	if err != nil {
		return nil, fmt.Errorf("tls connect: %w", err)
	}
	_ = conn.SetDeadline(time.Now().Add(tcpDeadln))
	tcpMu.Lock()
	id := tcpNext
	tcpNext++
	tcpConns[id] = conn
	tcpMu.Unlock()
	return &object.Integer{Value: int64(id)}, nil
}

// tcpReceive implements std.net.tcp_receive(handle, maxBytes) returning a string.
func tcpReceive(args []object.Object) (object.Object, error) {
	if len(args) != 2 {
		return nil, fmt.Errorf("std.net.tcp_receive expects (handle, max_bytes)")
	}
	conn, _, err := tcpLookup(args[0])
	if err != nil {
		return nil, err
	}
	maxObj, ok := args[1].(*object.Integer)
	if !ok || maxObj.Value < 1 || maxObj.Value > 16<<20 {
		return nil, fmt.Errorf("tcp max_bytes must be an integer in [1, 16777216]")
	}
	_ = conn.SetDeadline(time.Now().Add(tcpDeadln))
	buf := make([]byte, int(maxObj.Value))
	n, err := conn.Read(buf)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("tcp receive: %w", err)
	}
	return &object.String{Value: string(buf[:n])}, nil
}

// tcpClose implements std.net.tcp_close(handle).
func tcpClose(args []object.Object) (object.Object, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("std.net.tcp_close expects (handle)")
	}
	conn, id, err := tcpLookup(args[0])
	if err != nil {
		return nil, err
	}
	_ = conn.Close()
	tcpMu.Lock()
	delete(tcpConns, id)
	tcpMu.Unlock()
	return &object.Null{}, nil
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
	case float32:
		return &object.Float{Value: float64(x)}
	case string:
		return &object.String{Value: x}
	case int64:
		return &object.Integer{Value: x}
	case int:
		return &object.Integer{Value: int64(x)}
	case int32:
		return &object.Integer{Value: int64(x)}
	case []byte:
		return &object.String{Value: string(x)}
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
