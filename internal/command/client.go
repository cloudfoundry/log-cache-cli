package command

import (
	"context"
	"time"

	logcache "code.cloudfoundry.org/go-log-cache/v2"
	"code.cloudfoundry.org/go-loggregator/v9/rpc/loggregator_v2"
)

var DefaultClient LogCacheClient

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . LogCacheClient
type LogCacheClient interface {
	Read(ctx context.Context, sourceID string, start time.Time, opts ...logcache.ReadOption) ([]*loggregator_v2.Envelope, error)
}
