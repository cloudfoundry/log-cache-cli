package command

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

func Meta(ctx context.Context, cli plugin.CliConnection, c HTTPClient, log Logger, tableWriter io.Writer) {
	logCacheEndpoint, err := logCacheEndpoint(cli)
	if err != nil {
		log.Fatalf("Couldn't determine LogCache endpoint: %s", err)
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
	_ = err

	apiEndpoint, err := cli.ApiEndpoint()
	_ = err

	capiRequest, err := http.NewRequest(
		http.MethodGet,
		apiEndpoint+"/v3/apps?guids="+sourceIDsFromMeta(meta),
		nil,
	)
	if err != nil {
		log.Fatalf("Couldn't build HTTP request: %s", err)
	}

	resp, err := c.Do(capiRequest)
	if err != nil {
		log.Fatalf("Couldn't perform HTTP request: %s", err)
	}

	var resources appsResponse
	err = json.NewDecoder(resp.Body).Decode(&resources)
	if err != nil {
		log.Fatalf("Couldn't decode CAPI response: %s", err)
	}

	username, err := cli.Username()
	if err != nil {
		log.Fatalf("Couldn't get username: %s", err)
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

func logCacheEndpoint(cli plugin.CliConnection) (string, error) {
	apiEndpoint, err := cli.ApiEndpoint()
	if err != nil {
		return "", err
	}
	return strings.Replace(apiEndpoint, "api", "log-cache", 1), nil
}

func sourceIDsFromMeta(meta map[string]*logcache.MetaInfo) string {
	var ids []string
	for id, _ := range meta {
		ids = append(ids, id)
	}
	return strings.Join(ids, ",")
}
