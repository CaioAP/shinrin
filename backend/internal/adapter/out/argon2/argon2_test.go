package argon2_test

import (
	"strings"
	"testing"

	"github.com/CaioAP/shinrin/backend/internal/adapter/out/argon2"
)

var cheap = argon2.Params{MemoryKiB: 1024, Time: 1, Threads: 1, SaltLen: 16, KeyLen: 32}

func TestHashAndVerify(t *testing.T) {
	h := argon2.New(cheap)
	enc, err := h.Hash("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(enc, "$argon2id$v=19$m=1024,t=1,p=1$") {
		t.Errorf("hash = %q", enc)
	}
	if again, _ := h.Hash("correct horse battery"); again == enc {
		t.Error("two hashes of one password must differ (random salt)")
	}
	if ok, err := h.Verify("correct horse battery", enc); !ok || err != nil {
		t.Errorf("right password: %v, %v", ok, err)
	}
	if ok, _ := h.Verify("wrong horse battery", enc); ok {
		t.Error("wrong password verified")
	}
	// A hash made with other parameters still verifies.
	if ok, _ := argon2.New(argon2.DefaultParams).Verify("correct horse battery", enc); !ok {
		t.Error("verify must use the parameters stored in the hash")
	}
}

func TestVerifyRejectsMalformed(t *testing.T) {
	for _, bad := range []string{"", "plain", "$argon2i$v=19$m=1,t=1,p=1$c2FsdA$aGFzaA", "$argon2id$v=19$m=x$c2FsdA$aGFzaA"} {
		if _, err := argon2.New(cheap).Verify("pw", bad); err == nil {
			t.Errorf("Verify(%q) accepted", bad)
		}
	}
}
