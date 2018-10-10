package cf

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
	logcache "code.cloudfoundry.org/log-cache/client"
	logcache_v1 "code.cloudfoundry.org/log-cache/rpc/logcache_v1"
	flags "github.com/jessevdk/go-flags"
)

const (
	sourceTypeApplication sourceType = "application"
	sourceTypeService     sourceType = "service"
	sourceTypePlatform    sourceType = "platform"
	sourceTypeAll         sourceType = "all"
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

type calculator struct {
	ctx    context.Context
	cli    plugin.CliConnection
	c      HTTPClient
	log    Logger
	tailer Tailer
}

func newCalculator(ctx context.Context, cli plugin.CliConnection, c HTTPClient, log Logger, tailer Tailer) *calculator {
	return &calculator{
		ctx:    ctx,
		cli:    cli,
		c:      c,
		log:    log,
		tailer: tailer,
	}
}

func (calc *calculator) rate(sourceID string) int {
	batch := struct {
		Results []string `json:"batch"`
	}{}

	var results []string

	for _, lines := range calc.tailer(sourceID) {
		json.NewDecoder(strings.NewReader(lines)).Decode(&batch)
		results = append(results, batch.Results...)
	}

	return len(results)
}

type optionsFlags struct {
	SourceType  string `long:"source-type"`
	EnableNoise bool   `long:"noise"`
	ShowGUID    bool   `long:"guid"`
	SortBy      string `long:"sort-by"`

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
		SortBy:      "source",
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

	sortBy := strings.ToLower(opts.SortBy)
	if invalidSortBy(sortBy) {
		log.Fatalf("Sort by must be 'source-id', 'source', 'source-type', 'count', 'expired', 'cache-duration', or 'rate'.")
	}

	if sortByRate.Equal(sortBy) && !opts.EnableNoise {
		log.Fatalf("Can't sort by rate column without --noise flag")
	}

	if sortBySourceID.Equal(sortBy) && !opts.ShowGUID {
		log.Fatalf("Can't sort by source id column without --guid flag")
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

	if opts.ShowGUID {
		headerArgs = append([]interface{}{"Source ID"}, headerArgs...)
		headerFormat = "%s\t" + headerFormat
		tableFormat = "%s\t" + tableFormat
	}

	if opts.EnableNoise {
		headerArgs = append(headerArgs, "Rate")
		headerFormat = strings.Replace(headerFormat, "\n", "\t%s\n", 1)
		tableFormat = strings.Replace(tableFormat, "\n", "\t%s\n", 1)
	}

	tw := tabwriter.NewWriter(tableWriter, 0, 2, 2, ' ', 0)
	if !opts.noHeaders {
		fmt.Fprintf(tw, headerFormat, headerArgs...)
	}
	var rows [][]interface{}
	calculator := newCalculator(ctx, cli, c, log, tailer)

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
				args = append(args, displayRate(calculator.rate(source.GUID)))
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
					args = append(args, displayRate(calculator.rate(sourceID)))
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
					args = append(args, displayRate(calculator.rate(sourceID)))
				}

				rows = append(rows, args)
			}
		}
	}

	sortRows(opts, rows)

	for _, r := range rows {
		fmt.Fprintf(tw, tableFormat, r...)
	}

	if err = tw.Flush(); err != nil {
		log.Fatalf("Error writing results")
	}
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

func sortRows(opts optionsFlags, rows [][]interface{}) {
	var sorter sort.Interface
	var columnPadding int

	// if we're sending the --guid flag, we prepend the source id column,
	// which pushes over all the other columns by 1
	if opts.ShowGUID {
		columnPadding += 1
	}

	switch opts.SortBy {
	case string(sortBySourceID):
		sorter = newColumnSorterWithLesser(&sourceLesser{
			colToSortOn: 0,
			rows:        rows,
		}, rows)
	case string(sortBySource):
		sorter = newColumnSorterWithLesser(&sourceLesser{
			colToSortOn: 0 + columnPadding,
			rows:        rows,
		}, rows)
	case string(sortBySourceType):
		sorter = newColumnSorter(1+columnPadding, rows)
	case string(sortByCount):
		sorter = newColumnSorter(2+columnPadding, rows)
	case string(sortByExpired):
		sorter = newColumnSorter(3+columnPadding, rows)
	case string(sortByCacheDuration):
		sorter = newColumnSorter(4+columnPadding, rows)
	case string(sortByRate):
		sorter = newColumnSorter(5+columnPadding, rows)
	default:
		sorter = newColumnSorterWithLesser(&sourceLesser{
			colToSortOn: 0 + columnPadding,
			rows:        rows,
		}, rows)
	}

	sort.Sort(sorter)
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
	validSourceTypes := []sourceType{
		sourceTypePlatform,
		sourceTypeApplication,
		sourceTypeService,
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

func (s *columnLesser) Less(i, j int) bool {
	if sourceI, ok := s.rows[i][s.colToSortOn].(int); ok {
		sourceJ := s.rows[j][s.colToSortOn].(int)

		return sourceI < sourceJ
	}

	if sourceI, ok := s.rows[i][s.colToSortOn].(int64); ok {
		sourceJ := s.rows[j][s.colToSortOn].(int64)

		return sourceI < sourceJ
	}

	if sourceI, ok := s.rows[i][s.colToSortOn].(string); ok {
		sourceJ := s.rows[j][s.colToSortOn].(string)

		// We might be sorting a rate that is ">999", which will return an
		// error when we try to convert to an integer. Catch those rates and
		// explicitly treat those as the greater value, returning true or
		// false as appropriate depending on which side of the comparison it
		// falls on.
		numSourceI, err := strconv.Atoi(sourceI)
		if err != nil {
			return false
		}

		numSourceJ, err := strconv.Atoi(sourceJ)
		if err != nil {
			return true
		}

		return numSourceI < numSourceJ
	}

	if sourceI, ok := s.rows[i][s.colToSortOn].(time.Duration); ok {
		sourceJ := s.rows[j][s.colToSortOn].(time.Duration)

		return sourceI < sourceJ
	}

	if sourceI, ok := s.rows[i][s.colToSortOn].(sourceType); ok {
		sourceJ := s.rows[j][s.colToSortOn].(sourceType)

		return sourceI < sourceJ
	}

	return false
}

type lesser interface {
	Less(i, j int) bool
}

type columnLesser struct {
	colToSortOn int
	rows        [][]interface{}
}

type columnSorter struct {
	l    lesser
	rows [][]interface{}
}

func newColumnSorterWithLesser(l lesser, rows [][]interface{}) *columnSorter {
	return &columnSorter{
		l:    l,
		rows: rows,
	}
}

func newColumnSorter(colToSortOn int, rows [][]interface{}) *columnSorter {
	return &columnSorter{
		l: &columnLesser{
			colToSortOn: colToSortOn,
			rows:        rows,
		},
		rows: rows,
	}
}

func (s *columnSorter) Len() int {
	return len(s.rows)
}

func (s *columnSorter) Less(i, j int) bool {
	return s.l.Less(i, j)
}

func (s *columnSorter) Swap(i, j int) {
	t := s.rows[i]
	s.rows[i] = s.rows[j]
	s.rows[j] = t
}

type sourceLesser struct {
	colToSortOn int
	rows        [][]interface{}
}

func (s *sourceLesser) Less(i, j int) bool {
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
