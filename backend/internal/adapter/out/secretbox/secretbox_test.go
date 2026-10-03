package secretbox_test

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"testing"

	"github.com/CaioAP/shinrin/backend/internal/adapter/out/secretbox"
)

func newBox(t *testing.T) *secretbox.Box {
	t.Helper()
	k := make([]byte, 32)
	_, _ = rand.Read(k)
	b, err := secretbox.New(k)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestRoundTrip(t *testing.T) {
	b := newBox(t)
	secret, aad := []byte("sk-ant-api03-example"), []byte("user:7")
	sealed, err := b.Seal(secret, aad)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed, secret) {
		t.Fatal("sealed value contains the plaintext")
	}
	again, _ := b.Seal(secret, aad)
	if bytes.Equal(sealed, again) {
		t.Fatal("two seals of the same value are identical")
	}
	got, err := b.Open(sealed, aad)
	if err != nil || !bytes.Equal(got, secret) {
		t.Fatalf("Open = %q, %v", got, err)
	}
}

func TestOpenRejects(t *testing.T) {
	b := newBox(t)
	sealed, _ := b.Seal([]byte("sk-1"), []byte("user:1"))
	tampered := bytes.Clone(sealed)
	tampered[len(tampered)-1] ^= 1
	cases := map[string]struct {
		box    *secretbox.Box
		sealed []byte
		aad    string
	}{
		"other user":       {b, sealed, "user:2"},
		"tampered":         {b, tampered, "user:1"},
		"other master key": {newBox(t), sealed, "user:1"},
		"truncated":        {b, sealed[:20], "user:1"},
		"empty":            {b, nil, "user:1"},
	}
	for name, c := range cases {
		if _, err := c.box.Open(c.sealed, []byte(c.aad)); !errors.Is(err, secretbox.ErrOpen) {
			t.Errorf("%s: err = %v, want ErrOpen", name, err)
		}
	}
}

func TestParseKey(t *testing.T) {
	k := make([]byte, 32)
	_, _ = rand.Read(k)
	for _, s := range []string{base64.StdEncoding.EncodeToString(k), base64.RawURLEncoding.EncodeToString(k) + "\n"} {
		got, err := secretbox.ParseKey(s)
		if err != nil || !bytes.Equal(got, k) {
			t.Fatalf("ParseKey(%q) = %v", s, err)
		}
	}
	for _, s := range []string{"", "short", base64.StdEncoding.EncodeToString(k[:16])} {
		if _, err := secretbox.ParseKey(s); err == nil {
			t.Fatalf("ParseKey(%q) accepted", s)
		}
	}
}
