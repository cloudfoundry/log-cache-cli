package command

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	logcache "code.cloudfoundry.org/go-log-cache"
	"code.cloudfoundry.org/go-log-cache/rpc/logcache_v1"
	"github.com/spf13/cobra"
)

type Meta struct {
	*cobra.Command

	conf      Config
	timeout   time.Duration
	noHeaders bool
}

type MetaOption func(*Meta)

func WithMetaTimeout(d time.Duration) MetaOption {
	return func(m *Meta) {
		m.timeout = d
	}
}

func WithMetaNoHeaders() MetaOption {
	return func(m *Meta) {
		m.noHeaders = true
	}
}

func NewMeta(conf Config, opts ...MetaOption) *cobra.Command {
	m := &Meta{
		conf:    conf,
		timeout: 2 * time.Second,
	}
	m.Command = m.command()

	for _, o := range opts {
		o(m)
	}

	return m.Command
}

func (m *Meta) command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   os.Args[0],
		Short: "List cluster logs and metrics",
		RunE:  m.runE,
		Args:  cobra.NoArgs,
	}
	return cmd
}

func (m *Meta) runE(cmd *cobra.Command, args []string) error {
	client := logcache.NewClient(m.conf.Addr)
	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()
	meta, err := client.Meta(ctx)
	if err != nil {
		if err == io.EOF {
			return nil
		}
		return err
	}
	if len(meta) == 0 {
		return nil
	}
	rows := rows(meta)

	headerArgs := []interface{}{
		"Resource",
		"Type",
		"Namespace",
		"Count",
		"Expired",
		"Cache Duration",
	}
	headerFormat := "%s\t%s\t%s\t%s\t%s\t%s\n"
	rowFormat := "%s\t%s\t%s\t%d\t%d\t%s\n"

	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
	if !m.noHeaders {
		fmt.Fprintf(tw, headerFormat, headerArgs...)
	}

	for _, r := range rows {
		resource, kind, namespace := sourceParts(r.SourceID)
		fmt.Fprintf(
			tw,
			rowFormat,
			resource,
			kind,
			namespace,
			r.Count,
			r.Expired,
			maxDuration(time.Second, r.Duration),
		)
	}

	if err = tw.Flush(); err != nil {
		return errors.New("Error writing results")
	}

	return nil
}

func maxDuration(a, b time.Duration) time.Duration {
	if a < b {
		return b
	}
	return a
}

func sourceParts(sourceID string) (string, string, string) {
	parts := strings.SplitN(sourceID, "/", 3)
	if len(parts) != 3 {
		return sourceID, "", ""
	}
	return parts[2], parts[1], parts[0]
}

type row struct {
	SourceID string
	Count    int64
	Expired  int64
	Duration time.Duration
}

func rows(meta map[string]*logcache_v1.MetaInfo) []row {
	rows := make([]row, 0, len(meta))
	for k, v := range meta {
		rows = append(rows, row{
			SourceID: k,
			Count:    v.Count,
			Expired:  v.Expired,
			Duration: cacheDuration(v),
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].SourceID < rows[j].SourceID
	})
	return rows
}

func cacheDuration(m *logcache_v1.MetaInfo) time.Duration {
	new := time.Unix(0, m.NewestTimestamp)
	old := time.Unix(0, m.OldestTimestamp)
	return new.Sub(old).Truncate(time.Second)
}
