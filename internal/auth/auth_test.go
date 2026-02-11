package auth

import (
	"strings"
	"testing"
)

func TestHashPassword(t *testing.T) {
	pass := "secure-password"
	actual_hash, err := HashPassword(pass)

	if err != nil {
		t.Errorf("%v", err)
	}
	if !strings.Contains(actual_hash, "$argon2id$v=19$m=65536,t=1,p=32$") {
		t.Error("Expected parameters do not match")
	}
}

func TestCheckPasswordHash(t *testing.T) {
	pass, hash := "secure-password", "$argon2id$v=19$m=65536,t=1,p=32$pcnYWsWWhLxlkv4+yhJQAA$RzORfFaMXi6OYhHGxWed7wUXRQjRC3gHo1UFbUpknmE"
	match, err := CheckPasswordHash(pass, hash)

	if err != nil {
		t.Errorf("%v", err)
	}

	if !match {
		t.Errorf("Hash: %v does not given password %v", hash, pass)
	}
}
