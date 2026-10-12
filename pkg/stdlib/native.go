package stdlib

// Host capability names used by host-backed standard-library operations.
// Keep these strings stable: the VM capability checker receives them verbatim.
const (
	CapJSON       = "std.json"
	CapCrypto     = "std.crypto"
	CapNetwork    = "std.net"
	CapFilesystem = "std.fs"
	CapDatabase   = "std.db"
	CapTime       = "std.time"
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
	NativeDBSQLQuery = "std.db.query"
	NativeDBSQLExec  = "std.db.exec"
	NativeTimeSleep  = "std.time.sleep"
	NativeTimeNowISO = "std.time.now_iso"
	NativeTimeFormat = "std.time.format"
	NativeAudioPlay  = "std.audio.play"

	// L10 additions: symmetric/asymmetric crypto, URL coding, TCP sockets.
	NativeAESEncrypt   = "std.crypto.aes_gcm_encrypt"
	NativeAESDecrypt   = "std.crypto.aes_gcm_decrypt"
	NativeEdKeypair    = "std.crypto.ed25519_keypair"
	NativeEdSign       = "std.crypto.ed25519_sign"
	NativeEdVerify     = "std.crypto.ed25519_verify"
	NativeURLEncode    = "std.net.url_encode"
	NativeURLDecode    = "std.net.url_decode"
	NativeTCPConnect   = "std.net.tcp_connect"
	NativeTCPSend      = "std.net.tcp_send"
	NativeTCPReceive   = "std.net.tcp_receive"
	NativeTCPClose     = "std.net.tcp_close"
	NativeTLSConnect   = "std.net.tls_connect"
	NativeChannelPick  = "std.sync.select_recv"
	NativeHashKeys     = "std.data.hash_keys"
	NativeDeviceInfo   = "std.device.info"
	NativeMobileNotify = "std.mobile.notify"
)
