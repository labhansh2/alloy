package provider

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAPIKey(t *testing.T) {
	auth, err := APIKey("x-custom-key", "server-key")
	if err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest("GET", "/", nil)
	request.Header.Add("X-Custom-Key", "caller-key")
	request.Header.Add("X-Custom-Key", "another-caller-key")

	if err := auth(request); err != nil {
		t.Fatal(err)
	}

	if values := request.Header.Values("X-Custom-Key"); len(values) != 1 || values[0] != "server-key" {
		t.Fatal("server key did not replace caller values")
	}
}

func TestAPIKeyValidation(t *testing.T) {
	for _, header := range []string{"", "bad header", "bad:header", "bad\r\nheader", "clé"} {
		if _, err := APIKey(header, "secret-key"); err == nil || strings.Contains(err.Error(), "secret-key") {
			t.Error("invalid header accepted or error exposed credential")
		}
	}

	for _, key := range []string{"", "bad key", "bad\nkey", "bad\x00key", "nonascii-🙂"} {
		if _, err := APIKey("X-Api-Key", key); err == nil {
			t.Error("invalid key accepted")
		}
	}
}
