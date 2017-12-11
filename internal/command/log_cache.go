package command

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"code.cloudfoundry.org/cli/plugin"
)

// Logger is used for outputting log-cache results and errors
type Logger interface {
	Fatalf(format string, args ...interface{})
	Printf(format string, args ...interface{})
}

// HTTPClient is the client used for HTTP requests
type HTTPClient interface {
	Get(url string) (resp *http.Response, err error)
}

// LogCache will fetch the logs for a given application guid and write them to
// stdout.
func LogCache(cli plugin.CliConnection, args []string, c HTTPClient, log Logger) {
	f := flag.NewFlagSet("log-cache", flag.ContinueOnError)
	start := f.Int64("start-time", 0, "")
	end := f.Int64("end-time", 0, "")
	envelopeType := f.String("envelope-type", "", "")
	limit := f.Uint64("limit", 0, "")
	outputFormat := f.String("output-format", "", "")

	err := f.Parse(args)
	if err != nil {
		log.Fatalf("%s", err)
	}

	if len(f.Args()) != 1 {
		log.Fatalf("Expected 1 argument, got %d.", len(f.Args()))
	}

	if *start > *end && *end != 0 {
		log.Fatalf("Invalid date/time range. Ensure your start time is prior or equal the end time.")
	}

	if *limit > 1000 {
		log.Fatalf("Invalid limit value. It must be 1000 or less.")
	}

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

	query := url.Values{}
	if *start != 0 {
		query.Set("starttime", fmt.Sprintf("%d", *start))
	}

	if *end != 0 {
		query.Set("endtime", fmt.Sprintf("%d", *end))
	}

	if *envelopeType != "" {
		query.Set("envelopetype", *envelopeType)
	}

	if *limit != 0 {
		query.Set("limit", fmt.Sprintf("%d", *limit))
	}

	outputter, err := buildOutputter(log, *outputFormat)
	if err != nil {
		log.Fatalf("%s", err)
	}
	_ = outputter

	URL, err := url.Parse(strings.Replace(tokenURL, "api", "log-cache", 1))
	URL.Path = f.Args()[0]
	URL.RawQuery = query.Encode()

	resp, err := c.Get(URL.String())
	if err != nil {
		log.Fatalf("%s", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Expected 200 response code, but got %d.", resp.StatusCode)
	}

	outputter(resp.Body)
}

func buildOutputter(log Logger, format string) (func(io.Reader), error) {
	if format != "" {
		templ := template.New("OutputFormat")
		templ.Funcs(map[string]interface{}{
			"base64": func(s string) interface{} {
				data, err := base64.StdEncoding.DecodeString(s)
				if err != nil {
					log.Fatalf("%s failed to base64 decode: %s", s, err)
				}

				d := json.NewDecoder(bytes.NewReader(data))
				d.UseNumber()

				var m map[string]interface{}
				if err := d.Decode(&m); err != nil {
					return string(data)
				}

				return m
			},
		})

		_, err := templ.Parse(format)
		if err != nil {
			return nil, err
		}

		return func(r io.Reader) {
			d := json.NewDecoder(r)
			d.UseNumber()

			m := struct {
				Envelopes []map[string]interface{}
			}{}

			err = d.Decode(&m)
			if err != nil {
				log.Fatalf("invalid json: %s", err)
			}

			for _, e := range m.Envelopes {
				b := bytes.Buffer{}
				if err := templ.Execute(&b, e); err != nil {
					log.Fatalf("Output template parsed, but failed to execute: %s", err)
				}

				if b.Len() == 0 {
					continue
				}

				log.Printf("%s", b.String())
			}
		}, nil
	}

	return func(r io.Reader) {
		data, err := ioutil.ReadAll(r)
		if err != nil {
			log.Fatalf("%s", err)
		}

		log.Printf("%s", data)
	}, nil
}
