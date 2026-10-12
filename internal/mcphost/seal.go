package mcphost

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
)

// sealer signs small payloads so they can travel through a client — as a
// client id, or a hidden form field — and come back unchanged.
//
// It exists so the parts of the sign-in that anyone can start cost this
// process nothing. Registering a client needs no credential, so a registry of
// them would be memory an outsider can fill; a signed client id is not stored
// at all. The key is generated per process, which is also what makes a
// restart invalidate everything issued before it.
type sealer struct{ key []byte }

func newSealer() sealer {
	key := make([]byte, 32)
	rand.Read(key) // never fails: the runtime aborts if the system has no randomness
	return sealer{key: key}
}

// mac binds the signature to what the payload is for, so a client id cannot be
// replayed as an authorization request.
func (s sealer) mac(purpose string, payload []byte) []byte {
	h := hmac.New(sha256.New, s.key)
	h.Write([]byte(purpose))
	h.Write([]byte{0})
	h.Write(payload)
	return h.Sum(nil)
}

func (s sealer) seal(purpose string, value any) string {
	payload, _ := json.Marshal(value) // plain structs of strings and numbers
	return encode(payload) + "." + encode(s.mac(purpose, payload))
}

// open reports whether sealed was produced by seal for this purpose, and fills
// into when it was.
func (s sealer) open(purpose, sealed string, into any) bool {
	body, signature, found := strings.Cut(sealed, ".")
	if !found {
		return false
	}
	payload, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return false
	}
	given, err := base64.RawURLEncoding.DecodeString(signature)
	if err != nil {
		return false
	}
	if !hmac.Equal(given, s.mac(purpose, payload)) {
		return false
	}
	return json.Unmarshal(payload, into) == nil
}

func encode(raw []byte) string { return base64.RawURLEncoding.EncodeToString(raw) }

// randomToken returns a secret for a client to hold: 256 bits, URL-safe.
func randomToken() string {
	raw := make([]byte, 32)
	rand.Read(raw)
	return encode(raw)
}

// digest is how a token is remembered. The map holds this, never the token,
// so a dump of the process does not hand out working credentials.
func digest(token string) string {
	sum := sha256.Sum256([]byte(token))
	return encode(sum[:])
}
