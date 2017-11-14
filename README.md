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
   -start-time       Start of query range in UNIX seconds.
   -end-time         End of query range in UNIX seconds.
```

[log-cache]: https://code.cloudfoundry.org/log-cache-release
[cf-cli]: https://code.cloudfoundry.org/cli
