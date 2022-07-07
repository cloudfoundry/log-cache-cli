package platform

type SourceType string

func (st SourceType) Equal(value string) bool {
	return string(st) == value
}

const (
	ApplicationType SourceType = "application"
	ServiceType     SourceType = "service"
	PlatformType    SourceType = "platform"
	AllType         SourceType = "all"
	DefaultType     SourceType = "default"
	UnknownType     SourceType = "unknown"
)
