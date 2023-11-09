package logcache_test

import (
	"bytes"
	"testing"

	"code.cloudfoundry.org/cli/plugin/pluginfakes"
	"code.cloudfoundry.org/log-cache-cli/v4/internal/logcache"
)

func TestMetaCmd_Run(t *testing.T) {
	var b bytes.Buffer
	cmd := logcache.MetaCmd{}
	cmd.Run(&pluginfakes.FakeCliConnection{}, []string{})
}
