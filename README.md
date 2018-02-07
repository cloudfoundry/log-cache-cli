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
```

[log-cache]: https://code.cloudfoundry.org/log-cache-release
[cf-cli]: https://code.cloudfoundry.org/cli
