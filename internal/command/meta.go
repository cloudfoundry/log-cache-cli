package command

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"code.cloudfoundry.org/cli/plugin"
	logcache "code.cloudfoundry.org/go-log-cache"
	"code.cloudfoundry.org/go-log-cache/rpc/logcache_v1"
)

type app struct {
	GUID string `json:"guid"`
	Name string `json:"name"`
}

type appsResponse struct {
	Resources []app `json:"resources"`
}

// Meta returns the metadata from Log Cache
func Meta(ctx context.Context, cli plugin.CliConnection, args []string, c HTTPClient, log Logger, tableWriter io.Writer) {
	f := flag.NewFlagSet("log-cache", flag.ContinueOnError)
	scope := f.String("scope", "all", "")
	err := f.Parse(args)
	if err != nil {
		log.Fatalf("Could not parse flags: %s", err)
	}

	logCacheEndpoint, err := logCacheEndpoint(cli)
	if err != nil {
		log.Fatalf("Could not determine Log Cache endpoint: %s", err)
	}

	tc := &tokenHTTPClient{
		c:        c,
		getToken: cli.AccessToken,
	}

	client := logcache.NewClient(
		logCacheEndpoint,
		logcache.WithHTTPClient(tc),
	)

	meta, err := client.Meta(ctx)
	if err != nil {
		log.Fatalf("Failed to read Meta information: %s", err)
	}

	meta = truncate(50, meta)
	lines, err := cli.CliCommandWithoutTerminalOutput(
		"curl",
		"/v3/apps?guids="+sourceIDsFromMeta(meta),
	)
	if err != nil {
		log.Fatalf("Failed to make CAPI request: %s", err)
	}

	var resources appsResponse
	err = json.NewDecoder(strings.NewReader(strings.Join(lines, ""))).Decode(&resources)
	if err != nil {
		log.Fatalf("Could not decode CAPI response: %s", err)
	}

	username, err := cli.Username()
	if err != nil {
		log.Fatalf("Could not get username: %s", err)
	}

	fmt.Fprintf(tableWriter, fmt.Sprintf(
		"Retrieving log cache metadata as %s...\n\n",
		username,
	))

	tw := tabwriter.NewWriter(tableWriter, 0, 2, 2, ' ', 0)
	fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", "Source ID", "App Name", "Count", "Expired", "Cache Duration")

	for _, app := range resources.Resources {
		m := meta[app.GUID]
		delete(meta, app.GUID)
		if *scope == "applications" || *scope == "all" {
			fmt.Fprintf(tw, "%s\t%s\t%d\t%d\t%s\n", app.GUID, app.Name, m.Count, m.Expired, cacheDuration(m))
		}
	}

	if *scope == "platform" || *scope == "all" {
		for sourceID, m := range meta {
			fmt.Fprintf(tw, "%s\t%s\t%d\t%d\t%s\n", sourceID, "", m.Count, m.Expired, cacheDuration(m))
		}
	}

	tw.Flush()
}

func cacheDuration(m *logcache_v1.MetaInfo) time.Duration {
	new := time.Unix(0, m.NewestTimestamp)
	old := time.Unix(0, m.OldestTimestamp)
	return new.Sub(old).Truncate(time.Second)
}

func truncate(count int, entries map[string]*logcache_v1.MetaInfo) map[string]*logcache_v1.MetaInfo {
	truncated := make(map[string]*logcache_v1.MetaInfo)
	for k, v := range entries {
		if len(truncated) >= count {
			break
		}
		truncated[k] = v
	}
	return truncated
}

func logCacheEndpoint(cli plugin.CliConnection) (string, error) {
	apiEndpoint, err := cli.ApiEndpoint()
	if err != nil {
		return "", err
	}
	return strings.Replace(apiEndpoint, "api", "log-cache", 1), nil
}

func sourceIDsFromMeta(meta map[string]*logcache_v1.MetaInfo) string {
	var ids []string
	for id := range meta {
		ids = append(ids, id)
	}
	return strings.Join(ids, ",")
}
