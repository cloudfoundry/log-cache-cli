package cmd

type SourceOpt string

const (
	AllSources   SourceOpt = "all"
	AppsOnly     SourceOpt = "application"
	ServicesOnly SourceOpt = "service"
	PlatformOnly SourceOpt = "platform"
	UnknownOnly  SourceOpt = "unknown"
)

type SortOpt string

const (
	SourceID      SortOpt = "source-id"
	Source        SortOpt = "source"
	SourceType    SortOpt = "source-type"
	Count         SortOpt = "count"
	Expired       SortOpt = "expired"
	CacheDuration SortOpt = "cache-duration"
	Rate          SortOpt = "rate"
)
