package signing

// Well-known trusted release keys.
//
// OnuronTeamPublicKeyHex is the single definition of the Onuron team's
// Ed25519 release-signing public key. It used to be duplicated as a hex
// literal in the package verifier's built-in trust store and in the nilpkg
// default configuration; a key rotation then had to find every copy, and a
// missed one would silently keep trusting the retired key. Both places now
// reference these constants, so rotation touches exactly one definition.
const (
	// OnuronTeamKeyID is the trust-store key id of the Onuron release key.
	OnuronTeamKeyID = "onuron-team"

	// OnuronTeamPublicKeyHex is the hex-encoded Ed25519 public key.
	OnuronTeamPublicKeyHex = "d863fc2a699bc7733c621979ecef87e6958b5ff8d6c620e6c7c864739aaac7dd"
)
