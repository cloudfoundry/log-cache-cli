package command

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"code.cloudfoundry.org/cli/plugin"
	logcache "code.cloudfoundry.org/go-log-cache"

	"code.cloudfoundry.org/log-cache-cli/v4/internal/util/http"
	"code.cloudfoundry.org/log-cache-cli/v4/internal/util/query"
)

func Query(ctx context.Context, cli plugin.CliConnection, args []string, c http.Client) error {
	opts, err := query.NewOpts(args)
	if err != nil {
		return fmt.Errorf("incorrect param: %w", err)
	}

	apiEndpoint, err := cli.ApiEndpoint()
	if err != nil {
		return err
	}

	endpoint := strings.Replace(apiEndpoint, "api", "log-cache", 1)

	c = http.NewTokenClient(c, func() string {
		token, err := cli.AccessToken()
		if err != nil {
			panic(fmt.Sprintf("Unable to get Access Token: %s", err))
		}
		return token
	})

	client := logcache.NewClient(endpoint, logcache.WithHTTPClient(c))

	var o []logcache.PromQLOption
	var res *logcache.PromQLQueryResult
	if !opts.Start.IsZero() {
		o = append(o, logcache.WithPromQLStart(opts.Start))
		o = append(o, logcache.WithPromQLEnd(opts.End))
		o = append(o, logcache.WithPromQLStep(opts.Step))
		res, err = client.PromQLRangeRaw(context.Background(), opts.Query, o...)
	} else {
		if !opts.Time.IsZero() {
			o = append(o, logcache.WithPromQLTime(opts.Time))
		}
		res, err = client.PromQLRaw(context.Background(), opts.Query, o...)
	}

	if err != nil {
		return query.ClientError{Msg: err.Error()}
	}

	if res != nil && res.Status == "error" {
		return query.RequestError{Type: res.ErrorType, Msg: res.Error}
	}

	body, err := json.Marshal(res.Data)
	if err != nil {
		return query.MarshalError{Msg: res.Error}
	}
	log.Println(string(body))

	return nil
}
