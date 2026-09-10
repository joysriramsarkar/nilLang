package stdlib

// Host capability names used by host-backed standard-library operations.
// Keep these strings stable: the VM capability checker receives them verbatim.
const (
	CapJSON       = "std.json"
	CapCrypto     = "std.crypto"
	CapNetwork    = "std.net"
	CapFilesystem = "std.fs"
)

const (
	NativeJSONEncode = "std.json.encode"
	NativeJSONDecode = "std.json.decode"
	NativeBase64Enc  = "std.base64.encode"
	NativeBase64Dec  = "std.base64.decode"
	NativeSHA256     = "std.crypto.sha256"
	NativeSHA512     = "std.crypto.sha512"
	NativeHMACSHA256 = "std.crypto.hmac_sha256"
	NativeRandom     = "std.crypto.random_bytes"
	NativeHTTPGet    = "std.http.get"
	NativeHTTPPost   = "std.http.post"
)
