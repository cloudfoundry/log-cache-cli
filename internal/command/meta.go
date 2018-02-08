package command

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"code.cloudfoundry.org/cli/plugin"
	gologcache "code.cloudfoundry.org/go-log-cache"
	"code.cloudfoundry.org/go-log-cache/rpc/logcache"
)

type app struct {
	GUID string `json:"guid"`
	Name string `json:"name"`
}

type appsResponse struct {
	Resources []app `json:"resources"`
}

func Meta(ctx context.Context, cli plugin.CliConnection, args []string, c HTTPClient, log Logger, tableWriter io.Writer) {
	logCacheEndpoint, err := logCacheEndpoint(cli)
	if err != nil {
		log.Fatalf("Could not determine Log Cache endpoint: %s", err)
	}

	tc := &tokenHTTPClient{
		c:        c,
		getToken: cli.AccessToken,
	}

	client := gologcache.NewClient(
		logCacheEndpoint,
		gologcache.WithHTTPClient(tc),
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

	tw := tabwriter.NewWriter(tableWriter, 10, 2, 2, ' ', 0)
	fmt.Fprintf(tw, "%s\t%s\n", "Source ID", "App Name")

	for _, app := range resources.Resources {
		delete(meta, app.GUID)
		fmt.Fprintf(tw, "%s\t%s\n", app.GUID, app.Name)
	}

	for sourceID := range meta {
		fmt.Fprintf(tw, "%s\n", sourceID)
	}

	tw.Flush()
}

func truncate(count int, entries map[string]*logcache.MetaInfo) map[string]*logcache.MetaInfo {
	truncated := make(map[string]*logcache.MetaInfo)
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

func sourceIDsFromMeta(meta map[string]*logcache.MetaInfo) string {
	var ids []string
	for id := range meta {
		ids = append(ids, id)
	}
	return strings.Join(ids, ",")
}
