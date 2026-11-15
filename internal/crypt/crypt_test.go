package crypt

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
)

// Cross-implementation vector produced by the browser (Web Crypto) code
// path with a fixed key and IV. If this test fails, the CLI and the web
// UI are incompatible.
const (
	jsVectorKey     = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY" // "0123456789abcdef0123456789abcdef"
	jsVectorPayload = "obLD1OX2p7jJ0OHynCaHWdSkhU0dfUKUzvmV7nnUyeXNhepDeCID7nkFcfuyBbr9DnFzl6jXe1iVD5u8xAM-h0VaQAxHZvJcV6yu3mA"
	jsVectorText    = "hello cache22"
)

func TestDecryptJSVector(t *testing.T) {
	key, err := DecodeKey(jsVectorKey)
	if err != nil {
		t.Fatal(err)
	}
	env, err := DecryptPayload(jsVectorPayload, key)
	if err != nil {
		t.Fatal(err)
	}
	if env.V != 1 || env.Kind != "text" {
		t.Fatalf("bad envelope header: %+v", env)
	}
	if string(env.Data) != jsVectorText {
		t.Fatalf("data = %q, want %q", env.Data, jsVectorText)
	}
}

func TestRoundTrip(t *testing.T) {
	for _, tc := range []struct{ kind, name, text string }{
		{"text", "", "rahasia sekali baca"},
		{"text", "", "unicode âœ“ ç§˜å¯† ðŸ”"},
		{"file", "catatan rahasia.txt", "file body\nwith lines"},
		{"file", "very-long-name-ðŸ”¥.bin", string(bytes.Repeat([]byte{0x00, 0xff, 0x10}, 4096))},
	} {
		key, err := GenerateKey()
		if err != nil {
			t.Fatal(err)
		}
		payload, err := EncryptPayload([]byte(tc.text), tc.kind, tc.name, key)
		if err != nil {
			t.Fatal(err)
		}
		env, err := DecryptPayload(payload, key)
		if err != nil {
			t.Fatal(err)
		}
		if env.Kind != tc.kind || env.Name != tc.name || !bytes.Equal(env.Data, []byte(tc.text)) {
			t.Fatalf("round trip mismatch: kind=%q name=%q len=%d", env.Kind, env.Name, len(env.Data))
		}
	}
}

func TestDecryptWithWrongKeyFails(t *testing.T) {
	key, _ := GenerateKey()
	payload, err := EncryptPayload([]byte("secret"), "text", "", key)
	if err != nil {
		t.Fatal(err)
	}
	wrong, _ := GenerateKey()
	if _, err := DecryptPayload(payload, wrong); !errors.Is(err, ErrDecrypt) {
		t.Fatalf("err = %v, want ErrDecrypt", err)
	}
}

func TestTamperedPayloadFails(t *testing.T) {
	key, _ := GenerateKey()
	payload, _ := EncryptPayload([]byte("secret"), "text", "", key)
	tampered := []byte(payload)
	tampered[10] ^= 0x01
	if _, err := DecryptPayload(string(tampered), key); !errors.Is(err, ErrDecrypt) {
		t.Fatalf("err = %v, want ErrDecrypt", err)
	}
}

func TestWrapUnwrapRoundTrip(t *testing.T) {
	key, _ := GenerateKey()
	mw, err := WrapKey(key, "s3cret-passphrase", 600_000)
	if err != nil {
		t.Fatal(err)
	}
	got, err := UnwrapKey(mw, "s3cret-passphrase")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, key) {
		t.Fatal("unwrapped key mismatch")
	}
	if _, err := UnwrapKey(mw, "wrong-passphrase"); !errors.Is(err, ErrDecrypt) {
		t.Fatalf("err = %v, want ErrDecrypt", err)
	}
}

// Known-answer test for PBKDF2-HMAC-SHA256.
func TestPBKDF2Vector(t *testing.T) {
	got := pbkdf2SHA256([]byte("password"), []byte("salt"), 1, 32)
	want := mustHex("120fb6cffcf8b32c43e7225256c4f837a86548c92ccc35480805987cb70be17b")
	if !bytes.Equal(got, want) {
		t.Fatalf("pbkdf2 = %x, want %x", got, want)
	}
}

func mustHex(s string) []byte {
	b := make([]byte, 0, len(s)/2)
	for i := 0; i+1 < len(s); i += 2 {
		var v int
		for _, c := range s[i : i+2] {
			v <<= 4
			switch {
			case c >= '0' && c <= '9':
				v |= int(c - '0')
			case c >= 'a' && c <= 'f':
				v |= int(c-'a') + 10
			}
		}
		b = append(b, byte(v))
	}
	return b
}

func TestEnvelopeJSONLayout(t *testing.T) {
	// The wire format is JSON; field names must stay exactly v, kind, name, data.
	env := envelopeJSON{V: 1, Kind: "file", Name: "n.txt", Data: "ZGF0YQ"}
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{`"v":1`, `"kind":"file"`, `"name":"n.txt"`, `"data":"ZGF0YQ"`} {
		if !bytes.Contains(b, []byte(want)) {
			t.Fatalf("envelope JSON %s missing %s", s, want)
		}
	}
}
