package logcache_test

import (
	"bytes"
	"io"
	"testing"

	"code.cloudfoundry.org/log-cache-cli/v4/internal/logcache"
)

func TestMetaCmd_Run(t *testing.T) {
	b := &bytes.Buffer{}
	cmd := logcache.MetaCmd{}
	cmd.SetOut(b)
	cmd.SetArgs([]string{"--in", "testisawesome"})
	cmd.Execute()
	out, err := io.ReadAll(b)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "testisawesome" {
		t.Fatalf("expected \"%s\" got \"%s\"", "testisawesome", string(out))
	}
}
