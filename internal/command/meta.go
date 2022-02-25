package command

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"code.cloudfoundry.org/cli/plugin"
	logcache "code.cloudfoundry.org/go-log-cache"
	logcache_v1 "code.cloudfoundry.org/go-log-cache/rpc/logcache_v1"
	flags "github.com/jessevdk/go-flags"
)

const (
	sourceTypeApplication sourceType = "application"
	sourceTypeService     sourceType = "service"
	sourceTypePlatform    sourceType = "platform"
	sourceTypeAll         sourceType = "all"
	sourceTypeDefault     sourceType = "default"
	sourceTypeUnknown     sourceType = "unknown"
	MaximumBatchSize      int        = 1000
)

type sourceType string

const (
	sortBySourceID      sortBy = "source-id"
	sortBySource        sortBy = "source"
	sortBySourceType    sortBy = "source-type"
	sortByCount         sortBy = "count"
	sortByExpired       sortBy = "expired"
	sortByCacheDuration sortBy = "cache-duration"
	sortByRate          sortBy = "rate"
)

type sortBy string

func (st sourceType) Equal(value string) bool {
	return string(st) == value
}

func (sb sortBy) Equal(value string) bool {
	return string(sb) == value
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

type Tailer func(sourceID string) []string

type optionsFlags struct {
	SourceType  string `long:"source-type"`
	EnableNoise bool   `long:"noise"`
	ShowGUID    bool   `long:"guid"`
	SortBy      string `long:"sort-by"`

	withHeaders            bool
	metaNoiseSleepDuration time.Duration
}

var (
	appOrServiceRegex = regexp.MustCompile("[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}")
)

type MetaOption func(*optionsFlags)

func WithMetaNoHeaders() MetaOption {
	return func(o *optionsFlags) {
		o.withHeaders = false
	}
}

func WithMetaNoiseSleepDuration(d time.Duration) MetaOption {
	return func(o *optionsFlags) {
		o.metaNoiseSleepDuration = d
	}
}

// Meta returns the metadata from Log Cache
func Meta(
	ctx context.Context,
	cli plugin.CliConnection,
	args []string,
	c HTTPClient,
	log Logger,
	tableWriter io.Writer,
	mopts ...MetaOption,
) {
	opts := getOptions(args, log, mopts...)
	client := createLogCacheClient(c, log, cli)
	tw := tabwriter.NewWriter(tableWriter, 0, 2, 2, ' ', 0)
	username, err := cli.Username()
	if err != nil {
		log.Fatalf("Could not get username: %s", err)
	}

	var originalMeta map[string]*logcache_v1.MetaInfo
	var currentMeta map[string]*logcache_v1.MetaInfo
	writeRetrievingMetaHeader(opts, tw, username)
	currentMeta, err = client.Meta(ctx)
	if err != nil {
		log.Fatalf("Failed to read Meta information: %s", err)
	}

	if opts.EnableNoise {
		originalMeta = currentMeta
		writeWaiting(opts, tw, username)
		time.Sleep(opts.metaNoiseSleepDuration)
		writeRetrievingMetaHeader(opts, tw, username)
		currentMeta, err = client.Meta(ctx)
		if err != nil {
			log.Fatalf("Failed to read Meta information: %s", err)
		}
	}

	resources := make(map[string]source)
	if !opts.ShowGUID {
		writeAppsAndServicesHeader(opts, tw, username)
		resources, err = getSourceInfo(currentMeta, cli)
		if err != nil {
			log.Fatalf("Failed to read application information: %s", err)
		}
	}

	writeHeaders(opts, tw, username)

	rows := toDisplayRows(resources, currentMeta, originalMeta)
	rows = filterRows(opts, rows)
	sortRows(opts, rows)

	for _, r := range rows {
		format, items := tableFormat(opts, r)
		fmt.Fprintf(tw, format, items...)
	}

	if err = tw.Flush(); err != nil {
		log.Fatalf("Error writing results")
	}
}

func toDisplayRows(resources map[string]source, currentMeta, originalMeta map[string]*logcache_v1.MetaInfo) []displayRow {
	var rows []displayRow
	for sourceID, m := range currentMeta {
		dR := displayRow{Source: sourceID, SourceID: sourceID, Count: m.Count, Expired: m.Expired, CacheDuration: cacheDuration(m)}
		source, isAppOrService := resources[sourceID]
		if isAppOrService {
			dR.Type = source.Type
			dR.Source = source.Name
		} else if appOrServiceRegex.MatchString(sourceID) {
			dR.Type = sourceTypeUnknown
		} else {
			dR.Type = sourceTypePlatform
		}
		if originalMeta[sourceID] != nil {
			diff := (m.Count + m.Expired) - (originalMeta[sourceID].Count + originalMeta[sourceID].Expired)
			dR.Delta = diff / 5
		} else {
			dR.Delta = -1
		}
		rows = append(rows, dR)
	}

	return rows
}

func filterRows(opts optionsFlags, rows []displayRow) []displayRow {
	if sourceTypeAll.Equal(opts.SourceType) {
		return rows
	}
	filteredRows := []displayRow{}
	for _, row := range rows {
		if row.Type == sourceTypeApplication && (sourceTypeApplication.Equal(opts.SourceType) || sourceTypeDefault.Equal(opts.SourceType)) {
			filteredRows = append(filteredRows, row)
		}
		if row.Type == sourceTypePlatform && (sourceTypePlatform.Equal(opts.SourceType) || sourceTypeDefault.Equal(opts.SourceType)) {
			filteredRows = append(filteredRows, row)
		}
		if row.Type == sourceTypeService && (sourceTypeService.Equal(opts.SourceType) || sourceTypeDefault.Equal(opts.SourceType)) {
			filteredRows = append(filteredRows, row)
		}
		if row.Type == sourceTypeUnknown && (sourceTypeUnknown.Equal(opts.SourceType) || shouldShowUknownWithGuidFlag(opts)) {
			filteredRows = append(filteredRows, row)
		}
	}
	return filteredRows
}

func shouldShowUknownWithGuidFlag(opts optionsFlags) bool {
	return opts.ShowGUID && !sourceTypePlatform.Equal(opts.SourceType)
}

type displayRow struct {
	Source        string
	SourceID      string
	Type          sourceType
	Count         int64
	Expired       int64
	CacheDuration time.Duration
	Delta         int64
}

func createLogCacheClient(c HTTPClient, log Logger, cli plugin.CliConnection) *logcache.Client {
	logCacheEndpoint, err := logCacheEndpoint(cli)
	if err != nil {
		log.Fatalf("Could not determine Log Cache endpoint: %s", err)
	}

	if strings.ToLower(os.Getenv("LOG_CACHE_SKIP_AUTH")) != "true" {
		c = &tokenHTTPClient{
			c: c,
			tokenFunc: func() string {
				token, err := cli.AccessToken()
				if err != nil {
					log.Fatalf("Unable to get Access Token: %s", err)
				}
				return token
			},
		}
	}

	return logcache.NewClient(
		logCacheEndpoint,
		logcache.WithHTTPClient(c),
	)
}

func tableFormat(opts optionsFlags, row displayRow) (string, []interface{}) {
	tableFormat := "%d\t%d\t%s\n"
	items := []interface{}{interface{}(row.Count), interface{}(row.Expired), interface{}(row.CacheDuration)}

	if opts.ShowGUID {
		tableFormat = "%s\t" + tableFormat
		items = append([]interface{}{interface{}(row.SourceID)}, items...)
	} else {
		tableFormat = "%s\t%s\t" + tableFormat
		items = append([]interface{}{interface{}(row.Source), interface{}(row.Type)}, items...)
	}

	if opts.EnableNoise {
		tableFormat = strings.Replace(tableFormat, "\n", "\t%d\n", 1)
		items = append(items, interface{}(row.Delta))
	}

	return tableFormat, items
}

func writeRetrievingMetaHeader(opts optionsFlags, tableWriter io.Writer, username string) {
	if opts.withHeaders {
		fmt.Fprintf(tableWriter, fmt.Sprintf(
			"Retrieving log cache metadata as %s...\n\n",
			username,
		))
	}
}

func writeAppsAndServicesHeader(opts optionsFlags, tableWriter io.Writer, username string) {
	if opts.withHeaders {
		fmt.Fprintf(tableWriter, fmt.Sprintf(
			"Retrieving app and service names as %s...\n\n",
			username,
		))
	}
}

func writeHeaders(opts optionsFlags, tableWriter io.Writer, username string) {
	if opts.withHeaders {
		headerArgs := []interface{}{"Count", "Expired", "Cache Duration"}
		headerFormat := "%s\t%s\t%s\n"

		if opts.ShowGUID {
			headerArgs = append([]interface{}{"Source ID"}, headerArgs...)
			headerFormat = "%s\t" + headerFormat
		} else {
			headerArgs = append([]interface{}{"Source", "Source Type"}, headerArgs...)
			headerFormat = "%s\t%s\t" + headerFormat
		}

		if opts.EnableNoise {
			headerArgs = append(headerArgs, "Rate/minute")
			headerFormat = strings.Replace(headerFormat, "\n", "\t%s\n", 1)
		}
		fmt.Fprintf(tableWriter, headerFormat, headerArgs...)
	}

}

func writeWaiting(opts optionsFlags, tableWriter io.Writer, username string) {
	if opts.withHeaders {
		fmt.Fprintf(tableWriter, "Waiting 5 minutes then comparing log output...\n\n")
	}
}

func getOptions(args []string, log Logger, mopts ...MetaOption) optionsFlags {
	opts := optionsFlags{
		SourceType:             "default",
		EnableNoise:            false,
		ShowGUID:               false,
		SortBy:                 "",
		withHeaders:            true,
		metaNoiseSleepDuration: 5 * time.Minute,
	}

	for _, o := range mopts {
		o(&opts)
	}

	args, err := flags.ParseArgs(&opts, args)
	if err != nil {
		log.Fatalf("Could not parse flags: %s", err)
	}

	if len(args) > 0 {
		log.Fatalf("Invalid arguments, expected 0, got %d.", len(args))
	}

	opts.SourceType = strings.ToLower(opts.SourceType)
	opts.SortBy = strings.ToLower(opts.SortBy)

	if opts.ShowGUID && (sortBySource.Equal(opts.SortBy) || sortBySourceType.Equal(opts.SortBy)) {
		log.Fatalf("When using --guid, sort by must be 'source-id', 'count', 'expired', 'cache-duration', or 'rate'.")
	}

	// validate what was entered before setting defaults
	if opts.SortBy == "" {
		opts.SortBy = string(sortBySource)
		if opts.ShowGUID {
			opts.SortBy = string(sortBySourceID)
		}
	}

	if opts.ShowGUID && !(sourceTypePlatform.Equal(opts.SourceType) || sourceTypeAll.Equal(opts.SourceType) || sourceTypeDefault.Equal(opts.SourceType)) {
		log.Fatalf("Source type must be 'platform' when using the --guid flag")
	}

	if invalidSourceType(opts.SourceType) {
		log.Fatalf("Source type must be 'platform', 'application', 'service', or 'all'.")
	}

	if invalidSortBy(opts.SortBy) {
		log.Fatalf("Sort by must be 'source-id', 'source', 'source-type', 'count', 'expired', 'cache-duration', or 'rate'.")
	}

	if sortByRate.Equal(opts.SortBy) && !opts.EnableNoise {
		log.Fatalf("Can't sort by rate column without --noise flag")
	}

	return opts
}

func displayRate(rate int) string {
	var output string

	if rate >= MaximumBatchSize {
		output = fmt.Sprintf(">%d", MaximumBatchSize-1)
	} else {
		output = strconv.Itoa(rate)
	}

	return output
}

func sortRows(opts optionsFlags, rows []displayRow) {
	switch opts.SortBy {
	case string(sortBySourceID):
		sort.Slice(rows, func(i, j int) bool {
			if rows[i].Type == sourceTypeUnknown && rows[j].Type != sourceTypeUnknown {
				return false
			}
			if rows[j].Type == sourceTypeUnknown && rows[i].Type != sourceTypeUnknown {
				return true
			}
			return rows[i].SourceID < rows[j].SourceID
		})
	case string(sortBySource):
		sort.Slice(rows, func(i, j int) bool {
			if rows[i].Type == sourceTypeUnknown && rows[j].Type != sourceTypeUnknown {
				return false
			}
			if rows[j].Type == sourceTypeUnknown && rows[i].Type != sourceTypeUnknown {
				return true
			}
			return rows[i].Source < rows[j].Source
		})
	case string(sortBySourceType):
		sort.Slice(rows, func(i, j int) bool {
			return rows[i].Type < rows[j].Type
		})
	case string(sortByCount):
		sort.Slice(rows, func(i, j int) bool {
			return rows[i].Count < rows[j].Count
		})
	case string(sortByExpired):
		sort.Slice(rows, func(i, j int) bool {
			return rows[i].Expired < rows[j].Expired
		})
	case string(sortByCacheDuration):
		sort.Slice(rows, func(i, j int) bool {
			return rows[i].CacheDuration < rows[j].CacheDuration
		})
	case string(sortByRate):
		sort.Slice(rows, func(i, j int) bool {
			return rows[i].Delta < rows[j].Delta
		})
	}
}

func getSourceInfo(metaInfo map[string]*logcache_v1.MetaInfo, cli plugin.CliConnection) (map[string]source, error) {
	var (
		resources map[string]source
		sourceIDs []string
	)
	resources = make(map[string]source)

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
			resources[res.GUID] = res
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
			resources[res.Metadata.GUID] = source{
				GUID: res.Metadata.GUID,
				Name: res.Entity.Name,
				Type: sourceTypeService,
			}
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
	validSourceTypes := []sourceType{
		sourceTypePlatform,
		sourceTypeApplication,
		sourceTypeService,
		sourceTypeUnknown,
		sourceTypeDefault,
		sourceTypeAll,
	}

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

func invalidSortBy(sb string) bool {
	validSortBy := []sortBy{
		sortBySourceID,
		sortBySource,
		sortBySourceType,
		sortByCount,
		sortByExpired,
		sortByCacheDuration,
		sortByRate,
	}

	if sb == "" {
		return false
	}

	for _, s := range validSortBy {
		if s.Equal(sb) {
			return false
		}
	}

	return true
}
