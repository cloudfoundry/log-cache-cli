package command

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"text/tabwriter"
	"time"

	"code.cloudfoundry.org/cli/plugin"
	logcache "code.cloudfoundry.org/go-log-cache"
	"code.cloudfoundry.org/go-log-cache/rpc/logcache_v1"
	flags "github.com/jessevdk/go-flags"
)

type app struct {
	GUID string `json:"guid"`
	Name string `json:"name"`
}

type appsResponse struct {
	Resources []app `json:"resources"`
}

type Tailer func(sourceID string, start, end time.Time) []string

type optionsFlags struct {
	Scope       string `long:"scope"`
	EnableNoise bool   `long:"noise"`
}

// Meta returns the metadata from Log Cache
func Meta(ctx context.Context, cli plugin.CliConnection, tailer Tailer, args []string, c HTTPClient, log Logger, tableWriter io.Writer) {
	opts := optionsFlags{
		Scope:       "all",
		EnableNoise: false,
	}

	args, err := flags.ParseArgs(&opts, args)
	if err != nil {
		log.Fatalf("Could not parse flags: %s", err)
	}

	if len(args) > 0 {
		log.Fatalf("Invalid arguments, expected 0, got %d.", len(args))
	}

	scope := strings.ToLower(opts.Scope)
	if invalidScope(scope) {
		log.Fatalf("Scope must be 'platform', 'applications' or 'all'.")
	}

	logCacheEndpoint, err := logCacheEndpoint(cli)
	if err != nil {
		log.Fatalf("Could not determine Log Cache endpoint: %s", err)
	}

	if strings.ToLower(os.Getenv("LOG_CACHE_SKIP_AUTH")) != "true" {
		token, err := cli.AccessToken()
		if err != nil {
			log.Fatalf("Unable to get Access Token: %s", err)
		}

		c = &tokenHTTPClient{
			c:           c,
			accessToken: token,
		}
	}

	client := logcache.NewClient(
		logCacheEndpoint,
		logcache.WithHTTPClient(c),
	)

	meta, err := client.Meta(ctx)
	if err != nil {
		log.Fatalf("Failed to read Meta information: %s", err)
	}

	resources, err := getAppInfo(meta, cli)
	if err != nil {
		log.Fatalf("Failed to read application information: %s", err)
	}

	username, err := cli.Username()
	if err != nil {
		log.Fatalf("Could not get username: %s", err)
	}

	fmt.Fprintf(tableWriter, fmt.Sprintf(
		"Retrieving log cache metadata as %s...\n\n",
		username,
	))

	headerArgs := []interface{}{"Source ID", "App Name", "Count", "Expired", "Cache Duration"}
	headerFormat := "%s\t%s\t%s\t%s\t%s\n"
	tableFormat := "%s\t%s\t%d\t%d\t%s\n"

	if opts.EnableNoise {
		headerArgs = append(headerArgs, "Rate")
		headerFormat = strings.Replace(headerFormat, "\n", "\t%s\n", 1)
		tableFormat = strings.Replace(tableFormat, "\n", "\t%d\n", 1)
	}

	tw := tabwriter.NewWriter(tableWriter, 0, 2, 2, ' ', 0)
	fmt.Fprintf(tw, headerFormat, headerArgs...)

	for _, app := range resources.Resources {
		m := meta[app.GUID]
		delete(meta, app.GUID)
		if scope == "applications" || scope == "all" {
			args := []interface{}{app.GUID, app.Name, m.Count, m.Expired, cacheDuration(m)}
			if opts.EnableNoise {
				end := time.Now()
				start := end.Add(-time.Minute)
				args = append(args, len(tailer(app.GUID, start, end)))
			}

			fmt.Fprintf(tw, tableFormat, args...)
		}
	}

	idRegexp := regexp.MustCompile("[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}")

	// Apps that do not have a known name from CAPI
	if scope == "applications" || scope == "all" {
		for sourceID, m := range meta {
			if idRegexp.MatchString(sourceID) {
				args := []interface{}{sourceID, "", m.Count, m.Expired, cacheDuration(m)}
				if opts.EnableNoise {
					end := time.Now()
					start := end.Add(-time.Minute)
					args = append(args, len(tailer(sourceID, start, end)))
				}
				fmt.Fprintf(tw, tableFormat, args...)
			}
		}
	}

	if scope == "platform" || scope == "all" {
		for sourceID, m := range meta {
			if !idRegexp.MatchString(sourceID) {
				args := []interface{}{sourceID, "", m.Count, m.Expired, cacheDuration(m)}
				if opts.EnableNoise {
					end := time.Now()
					start := end.Add(-time.Minute)
					args = append(args, len(tailer(sourceID, start, end)))
				}

				fmt.Fprintf(tw, tableFormat, args...)
			}
		}
	}

	tw.Flush()
}

func getAppInfo(meta map[string]*logcache_v1.MetaInfo, cli plugin.CliConnection) (appsResponse, error) {
	var (
		responseBodies []string
		resources      appsResponse
	)

	sourceIDs := sourceIDsFromMeta(meta)

	for len(sourceIDs) > 0 {
		n := 50
		if len(sourceIDs) < 50 {
			n = len(sourceIDs)
		}

		lines, err := cli.CliCommandWithoutTerminalOutput(
			"curl",
			"/v3/apps?guids="+strings.Join(sourceIDs[0:n], ","),
		)
		if err != nil {
			return appsResponse{}, err
		}

		sourceIDs = sourceIDs[n:]
		responseBodies = append(responseBodies, strings.Join(lines, ""))
	}

	for _, rb := range responseBodies {
		var r appsResponse
		err := json.NewDecoder(strings.NewReader(rb)).Decode(&r)
		if err != nil {
			return appsResponse{}, err
		}

		resources.Resources = append(resources.Resources, r.Resources...)
	}

	return resources, nil
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
	logCacheAddr := os.Getenv("LOG_CACHE_ADDR")

	if logCacheAddr != "" {
		return logCacheAddr, nil
	}

	apiEndpoint, err := cli.ApiEndpoint()
	if err != nil {
		return "", err
	}

	return strings.Replace(apiEndpoint, "api", "log-cache", 1), nil
}

func sourceIDsFromMeta(meta map[string]*logcache_v1.MetaInfo) []string {
	var ids []string
	for id := range meta {
		ids = append(ids, id)
	}

	return ids
}

func invalidScope(scope string) bool {
	validScopes := []string{"platform", "applications", "all"}

	if scope == "" {
		return false
	}

	for _, s := range validScopes {
		if scope == s {
			return false
		}
	}

	return true
}
