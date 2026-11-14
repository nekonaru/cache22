// Package crypt implements the CLI side of the Cache22 wire format.
// It mirrors the browser implementation exactly:
//
//	envelope := {"v":1,"kind":"text|file","name"?,"data":base64url(bytes)}
//	payload  := base64url( iv[12] || AES-256-GCM(envelope) )
//	fragment := base64url(rawKey[32])
//
// With a passphrase, rawKey is wrapped: KEK = PBKDF2-HMAC-SHA256(pass, salt,
// iter) and meta carries {salt, iterations, iv, wrapped}. Argon2 is not
// used because neither the Web Crypto API nor Go's standard library ships
// one; PBKDF2 with a high iteration count is the common denominator.
package crypt

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
)

const (
	Version       = 1
	keyBytes      = 32
	ivBytes       = 12
	saltBytes     = 16
	maxIterations = 5_000_000 // guard against hostile meta
	kindText      = "text"
	kindFile      = "file"
)

// ErrDecrypt indicates authentication failed (wrong key or tampering).
var ErrDecrypt = errors.New("crypt: decryption failed")

// Envelope is the decrypted plaintext structure (data is raw bytes).
type Envelope struct {
	V    int
	Kind string
	Name string
	Data []byte
}

// EnvelopeJSON mirrors the JS object exactly (data as base64url string).
type envelopeJSON struct {
	V    int    `json:"v"`
	Kind string `json:"kind"`
	Name string `json:"name,omitempty"`
	Data string `json:"data"`
}

// MetaWrap carries the passphrase-wrapping parameters stored server-side.
type MetaWrap struct {
	Salt       string `json:"salt"`
	Iterations int    `json:"iterations"`
	IV         string `json:"iv"`
	Wrapped    string `json:"wrapped"`
}

// GenerateKey returns a fresh random 32-byte key.
func GenerateKey() ([]byte, error) {
	k := make([]byte, keyBytes)
	if _, err := rand.Read(k); err != nil {
		return nil, err
	}
	return k, nil
}

// EncodeKey/DecodeKey map raw keys to fragment-safe base64url.
func EncodeKey(k []byte) string { return b64uEncode(k) }
func DecodeKey(s string) ([]byte, error) {
	k, err := b64uDecode(s)
	if err != nil || len(k) != keyBytes {
		return nil, errors.New("crypt: invalid key encoding")
	}
	return k, nil
}

// EncryptPayload encrypts the plaintext bytes as kind ("text"/"file") with
// an optional file name. It returns the payload (base64url) and the raw
// key (kept client-side only).
func EncryptPayload(plaintext []byte, kind, name string, key []byte) (payload string, err error) {
	if kind != kindText && kind != kindFile {
		return "", fmt.Errorf("crypt: invalid kind %q", kind)
	}
	if len(key) != keyBytes {
		return "", errors.New("crypt: key must be 32 bytes")
	}
	env := envelopeJSON{V: Version, Kind: kind, Name: name, Data: b64uEncode(plaintext)}
	plain, err := json.Marshal(env)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	iv := make([]byte, ivBytes)
	if _, err := rand.Read(iv); err != nil {
		return "", err
	}
	ct := gcm.Seal(nil, iv, plain, nil)
	payloadBytes := append(append([]byte{}, iv...), ct...)
	return b64uEncode(payloadBytes), nil
}

// DecryptPayload reverses EncryptPayload. On authentication failure it
// returns ErrDecrypt.
func DecryptPayload(payload string, key []byte) (Envelope, error) {
	var out Envelope
	if len(key) != keyBytes {
		return out, errors.New("crypt: key must be 32 bytes")
	}
	payloadBytes, err := b64uDecode(payload)
	if err != nil {
		return out, ErrDecrypt
	}
	if len(payloadBytes) <= ivBytes {
		return out, ErrDecrypt
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return out, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return out, err
	}
	iv, ct := payloadBytes[:ivBytes], payloadBytes[ivBytes:]
	plain, err := gcm.Open(nil, iv, ct, nil)
	if err != nil {
		return out, ErrDecrypt
	}
	var env envelopeJSON
	if err := json.Unmarshal(plain, &env); err != nil {
		return out, ErrDecrypt
	}
	if env.V != Version {
		return out, ErrDecrypt
	}
	if env.Kind != kindText && env.Kind != kindFile {
		return out, ErrDecrypt
	}
	data, err := b64uDecode(env.Data)
	if err != nil {
		return out, ErrDecrypt
	}
	return Envelope{V: env.V, Kind: env.Kind, Name: env.Name, Data: data}, nil
}

// WrapKey encrypts rawKey under a KEK derived from passphrase using
// PBKDF2-HMAC-SHA256. Returned parameters are safe to store server-side.
func WrapKey(rawKey []byte, passphrase string, iterations int) (MetaWrap, error) {
	var mw MetaWrap
	if iterations <= 0 || iterations > maxIterations {
		return mw, fmt.Errorf("crypt: iterations out of range")
	}
	salt := make([]byte, saltBytes)
	if _, err := rand.Read(salt); err != nil {
		return mw, err
	}
	kek := pbkdf2SHA256([]byte(passphrase), salt, iterations, keyBytes)
	block, err := aes.NewCipher(kek)
	if err != nil {
		return mw, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return mw, err
	}
	iv := make([]byte, ivBytes)
	if _, err := rand.Read(iv); err != nil {
		return mw, err
	}
	wrapped := gcm.Seal(nil, iv, rawKey, nil)
	return MetaWrap{
		Salt:       b64uEncode(salt),
		Iterations: iterations,
		IV:         b64uEncode(iv),
		Wrapped:    b64uEncode(wrapped),
	}, nil
}

// UnwrapKey reverses WrapKey.
func UnwrapKey(mw MetaWrap, passphrase string) ([]byte, error) {
	if mw.Iterations <= 0 || mw.Iterations > maxIterations {
		return nil, errors.New("crypt: iterations out of range")
	}
	salt, err := b64uDecode(mw.Salt)
	if err != nil {
		return nil, ErrDecrypt
	}
	iv, err := b64uDecode(mw.IV)
	if err != nil {
		return nil, ErrDecrypt
	}
	wrapped, err := b64uDecode(mw.Wrapped)
	if err != nil {
		return nil, ErrDecrypt
	}
	kek := pbkdf2SHA256([]byte(passphrase), salt, mw.Iterations, keyBytes)
	block, err := aes.NewCipher(kek)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	rawKey, err := gcm.Open(nil, iv, wrapped, nil)
	if err != nil {
		return nil, ErrDecrypt
	}
	if len(rawKey) != keyBytes {
		return nil, ErrDecrypt
	}
	return rawKey, nil
}

// pbkdf2SHA256 is PBKDF2 with HMAC-SHA256, implemented on the standard
// library to avoid external dependencies.
func pbkdf2SHA256(password, salt []byte, iterations, keyLen int) []byte {
	hLen := sha256.Size
	numBlocks := (keyLen + hLen - 1) / hLen
	out := make([]byte, 0, numBlocks*hLen)
	var buf [4]byte
	for block := 1; block <= numBlocks; block++ {
		binary.BigEndian.PutUint32(buf[:], uint32(block))
		hmacVal := hmac.New(sha256.New, password)
		_, _ = hmacVal.Write(salt)
		_, _ = hmacVal.Write(buf[:])
		u := hmacVal.Sum(nil)
		t := append([]byte{}, u...)
		for i := 2; i <= iterations; i++ {
			hmacVal = hmac.New(sha256.New, password)
			_, _ = hmacVal.Write(u)
			u = hmacVal.Sum(nil)
			for j := range t {
				t[j] ^= u[j]
			}
		}
		out = append(out, t...)
	}
	return out[:keyLen]
}

func b64uEncode(b []byte) string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	var buf bytes.Buffer
	for i := 0; i < len(b); i += 3 {
		var n uint32
		switch {
		case i+2 < len(b):
			n = uint32(b[i])<<16 | uint32(b[i+1])<<8 | uint32(b[i+2])
			buf.WriteByte(alphabet[(n>>18)&63])
			buf.WriteByte(alphabet[(n>>12)&63])
			buf.WriteByte(alphabet[(n>>6)&63])
			buf.WriteByte(alphabet[n&63])
		case i+1 < len(b):
			n = uint32(b[i])<<16 | uint32(b[i+1])<<8
			buf.WriteByte(alphabet[(n>>18)&63])
			buf.WriteByte(alphabet[(n>>12)&63])
			buf.WriteByte(alphabet[(n>>6)&63])
		default:
			n = uint32(b[i]) << 16
			buf.WriteByte(alphabet[(n>>18)&63])
			buf.WriteByte(alphabet[(n>>12)&63])
		}
	}
	return buf.String()
}

func b64uDecode(s string) ([]byte, error) {
	if len(s) == 0 {
		return nil, errors.New("empty input")
	}
	rev := map[byte]int{}
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	for i := 0; i < len(alphabet); i++ {
		rev[alphabet[i]] = i
	}
	rev['='] = -1
	var out []byte
	var acc uint32
	var nbits uint
	for i := 0; i < len(s); i++ {
		c := s[i]
		v, ok := rev[c]
		if !ok {
			return nil, fmt.Errorf("invalid base64url byte %q", c)
		}
		if v == -1 {
			break // padding
		}
		acc = acc<<6 | uint32(v)
		nbits += 6
		if nbits >= 8 {
			nbits -= 8
			out = append(out, byte(acc>>nbits))
			acc &= (1 << nbits) - 1
		}
	}
	return out, nil
}
