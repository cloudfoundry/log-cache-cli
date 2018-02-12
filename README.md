Log Cache CLI Plugin
====================

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
   -lines            Number of envelopes to return. Default is 10.
   -follow           Output appended to stdout as logs are egressed.
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
