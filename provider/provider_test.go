package provider

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestBearerTokenValidation(t *testing.T) {
	for _, token := range []string{"", "bad token", "bad\nkey", "bad\x00key", "nonascii-🙂"} {
		if _, err := BearerToken(token); err == nil {
			t.Error("invalid bearer token accepted")
		}
	}
}

func TestConnectionDoesNotFormatCredentials(t *testing.T) {
	c := NewConnection("http://127.0.0.1:8080/alloy/providers/test/http/", "seeded-secret")

	encoded, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(fmt.Sprintf("%v %+v %#v %s", c, c, c, encoded), "seeded-secret") {
		t.Fatal("connection formatting exposed credential")
	}
}
