Log Cache CLI Plugin
====================
[![GoDoc][go-doc-badge]][go-doc] [![travis][travis-badge]][travis] [![slack.cloudfoundry.org][slack-badge]][loggregator-slack]

The Log Cache CLI Plugin is a [CF CLI](cf-cli) plugin for the [Log
Cache](log-cache) system.

### Installing Plugin

```
go get code.cloudfoundry.org/log-cache-cli
cf install-plugin $GOPATH/bin/log-cache-cli
```

### Usage

##### `log-cache`

```
$ cf log-cache guid --help
NAME:
   log-cache -

USAGE:
   log-cache [options] <app-guid>

OPTIONS:
   -start-time       Start of query range in UNIX nanoseconds.
   -end-time         End of query range in UNIX nanoseconds.
   -envelope-type    Envelope type filter. Available filters: 'log', 'counter', 'gauge', 'timer', and 'event'.
   -lines, -n        Number of envelopes to return. Default is 10.
   -follow, -f       Output appended to stdout as logs are egressed.
   -json             Output envelopes in JSON format.
   -counter-name     Counter name filter (implies --envelope-type=counter).
   -gauge-name       Gauge name filter (implies --envelope-type=gauge).
```

##### `log-cache-meta`

```
$ cf log-cache-meta --help
NAME:
   log-cache-meta -

USAGE:
   log-cache-meta
```

[log-cache]: https://code.cloudfoundry.org/log-cache-release
[cf-cli]: https://code.cloudfoundry.org/cli

[slack-badge]:              https://slack.cloudfoundry.org/badge.svg
[loggregator-slack]:        https://cloudfoundry.slack.com/archives/loggregator
[go-doc-badge]:             https://godoc.org/code.cloudfoundry.org/log-cache-cli?status.svg
[go-doc]:                   https://godoc.org/code.cloudfoundry.org/log-cache-cli
[travis-badge]:             https://travis-ci.org/cloudfoundry-incubator/log-cache-cli.svg?branch=master
[travis]:                   https://travis-ci.org/cloudfoundry-incubator/log-cache-cli?branch=master
