package main

import (
	"crypto/tls"
	"log"
	"net/http"
	"os"

	"code.cloudfoundry.org/cli/plugin"
	"code.cloudfoundry.org/log-cache-cli/internal/command"
)

type LogCacheCLI struct{}

func (c *LogCacheCLI) Run(conn plugin.CliConnection, args []string) {
	if len(args) == 0 {
		log.Fatalf("Expected atleast 1 argument, but got 0.")
	}

	skipSSL, err := conn.IsSSLDisabled()
	if err != nil {
		log.Fatalf("%s", err)
	}
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{
		InsecureSkipVerify: skipSSL,
	}

	switch args[0] {
	case "log-cache":
		command.LogCache(
			conn,
			args[1:],
			http.DefaultClient,
			log.New(os.Stdout, "", 0),
		)
		return
	}
}

func (c *LogCacheCLI) GetMetadata() plugin.PluginMetadata {
	return plugin.PluginMetadata{
		Name: "Log Cache CLI Plugin",
		Commands: []plugin.Command{
			{
				Name: "log-cache",
				UsageDetails: plugin.Usage{
					Usage: "log-cache [options] <app>",
					Options: map[string]string{
						"start-time":    "Start of query range in UNIX nanoseconds.",
						"end-time":      "End of query range in UNIX nanoseconds.",
						"envelope-type": "Envelope type filter. Available filters: 'log', 'counter', 'gauge', 'timer', and 'event'.",
						"limit":         "Limit the number of envelopes to return. Defaults to 100. Max value is 1000.",
						"recent":        "Show all logs in cache for the app. If given, all other flags are ignored.",
						"raw-guid":      "Do not convert app name into a guid.",
					},
				},
			},
		},
	}
}

func main() {
	plugin.Start(&LogCacheCLI{})
}
