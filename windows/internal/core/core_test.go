package core

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"
)

func TestConfigTemplateQuotesWindowsPaths(t *testing.T) {
	yaml := ConfigTemplate(9000, "tg-abc", `C:\Users\o'neil\AppData\Local\CLI Tollgate\auths`)
	if !strings.Contains(yaml, `auth-dir: 'C:/Users/o''neil/AppData/Local/CLI Tollgate/auths'`) &&
		!strings.Contains(yaml, `auth-dir: 'C:\Users\o''neil\AppData\Local\CLI Tollgate\auths'`) {
		t.Fatalf("auth-dir not single-quoted:\n%s", yaml)
	}
	if parsePort(yaml) != 9000 {
		t.Fatalf("port not parsed back")
	}
}

func TestParsePort(t *testing.T) {
	cases := map[string]int{
		"server:\n  host: x\n  port: 8400\n":                8400,
		"server:\r\n  port: \"8401\" # comment\r\n":         8401,
		"other:\n  port: 1\nserver:\n  tls:\n    port: 9\n": DefaultPort,
		"server:\n  tls:\n    port: 9\n  port: 8402\n":      8402,
		"garbage": DefaultPort,
	}
	for in, want := range cases {
		if got := parsePort(in); got != want {
			t.Errorf("parsePort(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestVerifyUpdate(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	old := UpdatePublicKey
	UpdatePublicKey = base64.StdEncoding.EncodeToString(pub)
	defer func() { UpdatePublicKey = old }()

	data := []byte("installer bytes")
	sum := sha256.Sum256(data)
	feed := &UpdateFeed{
		Length:    int64(len(data)),
		SHA256:    hex.EncodeToString(sum[:]),
		Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(priv, data)),
	}
	if err := VerifyUpdate(data, feed); err != nil {
		t.Fatalf("valid update rejected: %v", err)
	}
	tampered := append([]byte{}, data...)
	tampered[0] ^= 1
	feed.SHA256 = ""
	if err := VerifyUpdate(tampered, feed); err == nil {
		t.Fatal("tampered update accepted")
	}
}

func TestProductionKeyDecodes(t *testing.T) {
	b, err := base64.StdEncoding.DecodeString(UpdatePublicKey)
	if err != nil || len(b) != ed25519.PublicKeySize {
		t.Fatalf("bad key: %v len=%d", err, len(b))
	}
}
