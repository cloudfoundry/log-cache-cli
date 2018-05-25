package cf

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"code.cloudfoundry.org/cli/plugin"
	logcache "code.cloudfoundry.org/go-log-cache"
	"code.cloudfoundry.org/go-log-cache/rpc/logcache_v1"
	flags "github.com/jessevdk/go-flags"
)

const (
	sourceTypeApplication sourceType = "application"
	sourceTypeService     sourceType = "service"
	sourceTypePlatform    sourceType = "platform"
	sourceTypeAll         sourceType = "all"
	sourceTypeUnknown     sourceType = "unknown"
)

type sourceType string

func (st sourceType) Equal(value string) bool {
	return string(st) == value
}

type source struct {
	GUID string `json:"guid"`
	Name string `json:"name"`
	Type sourceType
}

type sourceInfo struct {
	Resources []source `json:"resources"`
}

type serviceInstance struct {
	Metadata struct {
		GUID string `json:"guid"`
	} `json:"metadata"`
	Entity struct {
		Name string `json:"name"`
	} `json:"entity"`
}

type servicesResponse struct {
	Resources []serviceInstance `json:"resources"`
}

// Tailer defines our interface for querying Log Cache
type Tailer func(sourceID string, start, end time.Time) []string

type optionsFlags struct {
	SourceType  string `long:"source-type"`
	EnableNoise bool   `long:"noise"`
	ShowGUID    bool   `long:"guid"`

	noHeaders bool
}

var (
	appOrServiceRegex = regexp.MustCompile("[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}")
)

type MetaOption func(*optionsFlags)

func WithMetaNoHeaders() MetaOption {
	return func(o *optionsFlags) {
		o.noHeaders = true
	}
}

// Meta returns the metadata from Log Cache
func Meta(
	ctx context.Context,
	cli plugin.CliConnection,
	tailer Tailer,
	args []string,
	c HTTPClient,
	log Logger,
	tableWriter io.Writer,
	mopts ...MetaOption,
) {
	opts := optionsFlags{
		SourceType:  "all",
		EnableNoise: false,
		ShowGUID:    false,
	}

	args, err := flags.ParseArgs(&opts, args)
	if err != nil {
		log.Fatalf("Could not parse flags: %s", err)
	}

	for _, o := range mopts {
		o(&opts)
	}

	if len(args) > 0 {
		log.Fatalf("Invalid arguments, expected 0, got %d.", len(args))
	}

	sourceType := strings.ToLower(opts.SourceType)
	if invalidSourceType(sourceType) {
		log.Fatalf("Source type must be 'platform', 'application', 'service', or 'all'.")
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

	resources, err := getSourceInfo(meta, cli)
	if err != nil {
		log.Fatalf("Failed to read application information: %s", err)
	}

	username, err := cli.Username()
	if err != nil {
		log.Fatalf("Could not get username: %s", err)
	}

	if !opts.noHeaders {
		fmt.Fprintf(tableWriter, fmt.Sprintf(
			"Retrieving log cache metadata as %s...\n\n",
			username,
		))
	}

	headerArgs := []interface{}{"Source", "Source Type", "Count", "Expired", "Cache Duration"}
	headerFormat := "%s\t%s\t%s\t%s\t%s\n"
	tableFormat := "%s\t%s\t%d\t%d\t%s\n"
	colToSortOn := 0

	if opts.ShowGUID {
		headerArgs = append([]interface{}{"Source ID"}, headerArgs...)
		headerFormat = "%s\t" + headerFormat
		tableFormat = "%s\t" + tableFormat
		colToSortOn = 1
	}

	if opts.EnableNoise {
		headerArgs = append(headerArgs, "Rate")
		headerFormat = strings.Replace(headerFormat, "\n", "\t%s\n", 1)
		tableFormat = strings.Replace(tableFormat, "\n", "\t%d\n", 1)
	}

	tw := tabwriter.NewWriter(tableWriter, 0, 2, 2, ' ', 0)
	if !opts.noHeaders {
		fmt.Fprintf(tw, headerFormat, headerArgs...)
	}
	var rows [][]interface{}

	for _, source := range resources {
		m, ok := meta[source.GUID]
		if !ok {
			continue
		}
		delete(meta, source.GUID)

		displayApplication := sourceTypeApplication.Equal(sourceType) && source.Type == sourceTypeApplication
		displayService := sourceTypeService.Equal(sourceType) && source.Type == sourceTypeService
		if sourceTypeAll.Equal(sourceType) || displayApplication || displayService {
			args := []interface{}{source.Name, source.Type, m.Count, m.Expired, cacheDuration(m)}
			if opts.ShowGUID {
				args = append([]interface{}{source.GUID}, args...)
			}
			if opts.EnableNoise {
				end := time.Now()
				start := end.Add(-time.Minute)
				args = append(args, len(tailer(source.GUID, start, end)))
			}

			rows = append(rows, args)
		}
	}

	// Source IDs that aren't apps or services
	if sourceTypeAll.Equal(sourceType) {
		for sourceID, m := range meta {
			if appOrServiceRegex.MatchString(sourceID) {
				args := []interface{}{sourceID, sourceTypeUnknown, m.Count, m.Expired, cacheDuration(m)}
				if opts.ShowGUID {
					args = append([]interface{}{sourceID}, args...)
				}
				if opts.EnableNoise {
					end := time.Now()
					start := end.Add(-time.Minute)
					args = append(args, len(tailer(sourceID, start, end)))
				}

				rows = append(rows, args)
			}
		}
	}

	if sourceTypePlatform.Equal(sourceType) || sourceTypeAll.Equal(sourceType) {
		for sourceID, m := range meta {
			if !appOrServiceRegex.MatchString(sourceID) {
				args := []interface{}{sourceID, sourceTypePlatform, m.Count, m.Expired, cacheDuration(m)}
				if opts.ShowGUID {
					args = append([]interface{}{sourceID}, args...)
				}
				if opts.EnableNoise {
					end := time.Now()
					start := end.Add(-time.Minute)
					args = append(args, len(tailer(sourceID, start, end)))
				}

				rows = append(rows, args)
			}
		}
	}

	sort.Sort(&rowSorter{colToSortOn: colToSortOn, rows: rows})

	for _, r := range rows {
		fmt.Fprintf(tw, tableFormat, r...)
	}

	if err = tw.Flush(); err != nil {
		log.Fatalf("Error writing results")
	}
}

func getSourceInfo(metaInfo map[string]*logcache_v1.MetaInfo, cli plugin.CliConnection) ([]source, error) {
	var (
		resources []source
		sourceIDs []string
	)

	meta := make(map[string]int)
	for k := range metaInfo {
		meta[k] = 1
		sourceIDs = append(sourceIDs, k)
	}

	appInfo, err := getSourceInfoFromCAPI(sourceIDs, "/v3/apps", cli)
	if err != nil {
		return nil, err
	}
	for _, rb := range appInfo {
		var r sourceInfo
		err := json.NewDecoder(strings.NewReader(rb)).Decode(&r)
		if err != nil {
			return nil, err
		}

		for _, res := range r.Resources {
			res.Type = sourceTypeApplication
			resources = append(resources, res)
		}
	}

	for _, res := range resources {
		delete(meta, res.GUID)
	}
	var s []string
	for id := range meta {
		s = append(s, id)
	}

	serviceInfo, err := getSourceInfoFromCAPI(s, "/v2/service_instances", cli)
	if err != nil {
		return nil, err
	}

	for _, rb := range serviceInfo {
		var r servicesResponse
		err := json.NewDecoder(strings.NewReader(rb)).Decode(&r)
		if err != nil {
			return nil, err
		}
		for _, res := range r.Resources {
			resources = append(resources, source{
				GUID: res.Metadata.GUID,
				Name: res.Entity.Name,
				Type: sourceTypeService,
			})
		}
	}

	return resources, nil
}

func getSourceInfoFromCAPI(sourceIDs []string, endpoint string, cli plugin.CliConnection) ([]string, error) {
	var responses []string
	for len(sourceIDs) > 0 {
		n := 50
		if len(sourceIDs) < 50 {
			n = len(sourceIDs)
		}

		lines, err := cli.CliCommandWithoutTerminalOutput(
			"curl",
			endpoint+"?guids="+strings.Join(sourceIDs[0:n], ","),
		)
		if err != nil {
			return nil, err
		}

		sourceIDs = sourceIDs[n:]
		rb := strings.Join(lines, "")
		responses = append(responses, rb)
	}
	return responses, nil
}

func cacheDuration(m *logcache_v1.MetaInfo) time.Duration {
	new := time.Unix(0, m.NewestTimestamp)
	old := time.Unix(0, m.OldestTimestamp)

	return maxDuration(time.Second, new.Sub(old).Truncate(time.Second))
}

func maxDuration(a, b time.Duration) time.Duration {
	if a < b {
		return b
	}
	return a
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

func invalidSourceType(st string) bool {
	validSourceTypes := []sourceType{sourceTypePlatform, sourceTypeApplication, sourceTypeAll, sourceTypeService}

	if st == "" {
		return false
	}

	for _, s := range validSourceTypes {
		if s.Equal(st) {
			return false
		}
	}

	return true
}

type rowSorter struct {
	colToSortOn int
	rows        [][]interface{}
}

func newRowSorter(colToSortOn int) *rowSorter {
	return &rowSorter{
		colToSortOn: colToSortOn,
	}
}

func (s *rowSorter) Len() int {
	return len(s.rows)
}

func (s *rowSorter) Less(i, j int) bool {
	sourceI := s.rows[i][s.colToSortOn].(string)
	sourceJ := s.rows[j][s.colToSortOn].(string)

	isGuidI := appOrServiceRegex.MatchString(sourceI)
	isGuidJ := appOrServiceRegex.MatchString(sourceJ)

	// Both are guids
	if isGuidI && isGuidJ {
		return sourceI < sourceJ
	}

	// Only sourceI is guid
	if isGuidI {
		return false
	}

	// Only sourceJ is guid
	if isGuidJ {
		return true
	}

	// Neither sourceI or sourceJ are guids
	return sourceI < sourceJ
}

func (s *rowSorter) Swap(i, j int) {
	t := s.rows[i]
	s.rows[i] = s.rows[j]
	s.rows[j] = t
}
