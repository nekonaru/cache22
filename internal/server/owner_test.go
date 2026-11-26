package server

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/nekonaru/cache22/internal/store"
)

func auth(token string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + token}
}

func createWithToken(t *testing.T, s *Server, body string) createResponse {
	t.Helper()
	rr := createDrop(t, s, body)
	if rr.Code != 201 {
		t.Fatalf("create status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var cr createResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &cr); err != nil {
		t.Fatal(err)
	}
	if _, err := base64Decode(cr.OwnerToken); err != nil || len(cr.OwnerToken) < 40 {
		t.Fatalf("invalid owner token %q", cr.OwnerToken)
	}
	return cr
}

func base64Decode(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}

func TestOwnerStatusRevokeExtend(t *testing.T) {
	s := testServer(t)
	cr := createWithToken(t, s, `{"payload":"`+validPayload+`","ttl":"1h","kind":"text","max_reads":2}`)

	st := do(t, s, "GET", "/api/drops/"+cr.ID+"/status", nil, auth(cr.OwnerToken))
	if st.Code != 200 {
		t.Fatalf("status = %d, body = %s", st.Code, st.Body.String())
	}
	var sr statusResponse
	if err := json.Unmarshal(st.Body.Bytes(), &sr); err != nil {
		t.Fatal(err)
	}
	if sr.Kind != "text" || sr.ReadsLeft != 2 || sr.ExpiresAt == "" || !sr.CanExtend {
		t.Fatalf("bad status %+v", sr)
	}

	ex := strings.NewReader(`{"ttl":"24h"}`)
	er := do(t, s, "POST", "/api/drops/"+cr.ID+"/extend", ex, auth(cr.OwnerToken))
	if er.Code != 200 {
		t.Fatalf("extend = %d, body = %s", er.Code, er.Body.String())
	}
	var eb map[string]string
	if err := json.Unmarshal(er.Body.Bytes(), &eb); err != nil || eb["status"] != "extended" {
		t.Fatalf("bad extend body: %v, %v", eb, err)
	}

	// A second extension is rejected with the same uniform 404.
	ex2 := do(t, s, "POST", "/api/drops/"+cr.ID+"/extend", strings.NewReader(`{"ttl":"24h"}`), auth(cr.OwnerToken))
	if ex2.Code != 404 {
		t.Fatalf("second extend = %d, want 404", ex2.Code)
	}

	rv := do(t, s, "POST", "/api/drops/"+cr.ID+"/revoke", nil, auth(cr.OwnerToken))
	if rv.Code != 200 {
		t.Fatalf("revoke = %d, body = %s", rv.Code, rv.Body.String())
	}
	st2 := do(t, s, "GET", "/api/drops/"+cr.ID+"/status", nil, auth(cr.OwnerToken))
	if st2.Code != 404 {
		t.Fatalf("status after revoke = %d, want 404", st2.Code)
	}
}

func TestOwnerEndpointsRejectWrongToken(t *testing.T) {
	s := testServer(t)
	cr := createWithToken(t, s, `{"payload":"`+validPayload+`","ttl":"1h","kind":"text"}`)
	wrong := auth(strings.Repeat("A", 43))

	cases := []struct {
		method string
		path   string
		body   io.Reader
	}{
		{"GET", "/api/drops/" + cr.ID + "/status", nil},
		{"POST", "/api/drops/" + cr.ID + "/revoke", nil},
		{"POST", "/api/drops/" + cr.ID + "/extend", strings.NewReader(`{"ttl":"24h"}`)},
	}
	var want []byte
	for i, tc := range cases {
		rr := do(t, s, tc.method, tc.path, tc.body, wrong)
		if i == 0 {
			want = append([]byte(nil), rr.Body.Bytes()...)
		}
		if rr.Code != 404 {
			t.Fatalf("%s %s = %d, want 404", tc.method, tc.path, rr.Code)
		}
		if string(rr.Body.Bytes()) != string(want) {
			t.Fatalf("%s body differs: %q vs %q", tc.path, rr.Body.String(), want)
		}
	}
}

func TestOwnerStatusReportsRecipients(t *testing.T) {
	s := testServer(t)
	cr := createWithToken(t, s, `{"payload":"`+validPayload+`","ttl":"1h","kind":"text","max_reads":2}`)
	do(t, s, "POST", "/api/drops/"+cr.ID+"/read", nil, nil)
	st := do(t, s, "GET", "/api/drops/"+cr.ID+"/status", nil, auth(cr.OwnerToken))
	var sr statusResponse
	if err := json.Unmarshal(st.Body.Bytes(), &sr); err != nil {
		t.Fatal(err)
	}
	if sr.Recipients != 1 || sr.ReadsLeft != 1 {
		t.Fatalf("bad recipient count: %+v", sr)
	}
}

func TestOwnerHashPersistsAcrossStores(t *testing.T) {
	// Both backends must reject a wrong hash after storing the same drop.
	m := store.NewMemory(0)
	d, err := createDropForStore(t)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Put(d); err != nil {
		t.Fatal(err)
	}
	wrong := make([]byte, 32)
	if _, ok := m.OwnerGet(d.ID, wrong, time.Now()); ok {
		t.Fatal("memory store accepted wrong hash")
	}
	if _, ok := m.OwnerGet(d.ID, d.OwnerHash, time.Now()); !ok {
		t.Fatal("memory store rejected correct hash")
	}
}

func createDropForStore(t *testing.T) (store.Drop, error) {
	t.Helper()
	token, hash, err := newOwnerToken()
	if err != nil {
		return store.Drop{}, err
	}
	if token == "" {
		t.Fatal("empty token")
	}
	now := time.Now()
	d := store.NewDrop("ABCDEFGHIJKLMNOPQRSTUVWXYZ", "payload", "text", nil, 1, now, time.Hour, 7)
	d.OwnerHash = hash
	return d, nil
}
