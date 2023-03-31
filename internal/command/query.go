package command

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"code.cloudfoundry.org/log-cache-cli/v4/internal/util/http"

	"code.cloudfoundry.org/cli/plugin"
	logcache "code.cloudfoundry.org/go-log-cache/v2"
	flags "github.com/jessevdk/go-flags"
)

type QueryOption func(*queryOptions)

func Query(
	cli plugin.CliConnection,
	args []string,
	c http.Client,
	log Logger,
	w io.Writer,
	opts ...QueryOption,
) {
	if len(args) < 1 {
		log.Fatalf("Must specify a PromQL query")
	}
	query := args[0]

	queryOptions, err := newQueryOptions(cli, args, log)
	if err != nil {
		log.Fatalf("%s", err)
	}

	for _, opt := range opts {
		opt(&queryOptions)
	}

	lw := lineWriter{w: w}

	if strings.ToLower(os.Getenv("LOG_CACHE_SKIP_AUTH")) != "true" {
		c = http.NewTokenClient(c, func() string {
			token, err := cli.AccessToken()
			if err != nil {
				log.Fatalf("Unable to get Access Token: %s", err)
			}
			return token
		})
	}

	logCacheAddr := os.Getenv("LOG_CACHE_ADDR")
	if logCacheAddr == "" {
		hasAPI, err := cli.HasAPIEndpoint()
		if err != nil {
			log.Fatalf("%s", err)
		}

		if !hasAPI {
			log.Fatalf("No API endpoint targeted.")
		}

		tokenURL, err := cli.ApiEndpoint()
		if err != nil {
			log.Fatalf("%s", err)
		}

		logCacheAddr = strings.Replace(tokenURL, "api", "log-cache", 1)
	}

	client := logcache.NewClient(logCacheAddr, logcache.WithHTTPClient(c))

	var res *logcache.PromQLQueryResult

	if !queryOptions.rangeQuery {
		var options []logcache.PromQLOption

		if queryOptions.timeProvided {
			options = append(options, logcache.WithPromQLTime(queryOptions.time))
		}

		res, err = client.PromQLRaw(
			context.Background(),
			query,
			options...,
		)
	} else {
		res, err = client.PromQLRangeRaw(
			context.Background(),
			query,
			logcache.WithPromQLStart(queryOptions.start),
			logcache.WithPromQLEnd(queryOptions.end),
			logcache.WithPromQLStep(queryOptions.step),
		)
	}

	if err != nil {
		lw.Write(fmt.Sprintf("Could not process query: %s", err.Error()))
		return
	}

	if res != nil && res.Status == "error" {
		lw.Write(fmt.Sprintf("The PromQL API returned an error (%s): %s", res.ErrorType, res.Error))
		return
	}

	body, _ := json.Marshal(res)
	lw.Write(string(body))
}

type queryOptions struct {
	time         time.Time
	start        time.Time
	end          time.Time
	step         string
	rangeQuery   bool
	timeProvided bool
}

type queryOptionFlags struct {
	Time  string `long:"time"`
	Start string `long:"start"`
	End   string `long:"end"`
	Step  string `long:"step"`
}

func newQueryOptions(cli plugin.CliConnection, args []string, log Logger) (queryOptions, error) {
	opts := queryOptionFlags{}

	args, err := flags.ParseArgs(&opts, args)
	if err != nil {
		return queryOptions{}, err
	}

	if len(args) != 1 {
		return queryOptions{}, fmt.Errorf("Expected 1 argument, got %d.", len(args))
	}

	if isInstantQuery(opts) && !validInstantQueryArgs(opts) {
		return queryOptions{}, errors.New("When issuing an instant query, you cannot specify --start, --end, or --step")
	}

	if isRangeQuery(opts) && !validRangeQueryArgs(opts) {
		return queryOptions{}, errors.New("When issuing a range query, you must specify all of --start, --end, and --step")
	}

	if isInstantQuery(opts) {
		if opts.Time == "" {
			return queryOptions{}, nil
		}

		parsedTime, err := getParsedTime(opts.Time)
		if err != nil {
			return queryOptions{}, fmt.Errorf("Couldn't parse --time: %s", err.Error())
		}

		return queryOptions{timeProvided: true, time: parsedTime}, nil
	}

	if isRangeQuery(opts) {
		parsedStart, err := getParsedTime(opts.Start)
		if err != nil {
			return queryOptions{}, fmt.Errorf("Couldn't parse --start: %s", err.Error())
		}
		parsedEnd, err := getParsedTime(opts.End)
		if err != nil {
			return queryOptions{}, fmt.Errorf("Couldn't parse --end: %s", err.Error())
		}

		return queryOptions{start: parsedStart, end: parsedEnd, step: opts.Step, rangeQuery: true}, nil
	}

	return queryOptions{}, nil
}

func getParsedTime(inputTime string) (time.Time, error) {
	if t, err := strconv.Atoi(inputTime); err == nil {
		return time.Unix(int64(t), 0), nil
	}
	if parsedTime, err := time.Parse(time.RFC3339, inputTime); err == nil {
		return parsedTime, nil
	}

	return time.Time{}, fmt.Errorf("invalid time format: %s", inputTime)
}

func isInstantQuery(opts queryOptionFlags) bool {
	return opts.Time != "" || (opts.Start == "" && opts.End == "" && opts.Step == "")
}

func validInstantQueryArgs(opts queryOptionFlags) bool {
	return opts.Start == "" && opts.End == "" && opts.Step == ""
}

func isRangeQuery(opts queryOptionFlags) bool {
	return opts.Time == "" && (opts.Start != "" || opts.End != "" || opts.Step != "")
}

func validRangeQueryArgs(opts queryOptionFlags) bool {
	return opts.Start != "" && opts.End != "" && opts.Step != ""
}
