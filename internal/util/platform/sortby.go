package platform

type SortBy string

const (
	SortBySourceID      SortBy = "source-id"
	SortBySource        SortBy = "source"
	SortBySourceType    SortBy = "source-type"
	SortByCount         SortBy = "count"
	SortByExpired       SortBy = "expired"
	SortByCacheDuration SortBy = "cache-duration"
	SortByRate          SortBy = "rate"
)

func (sb SortBy) Equal(value string) bool {
	return string(sb) == value
}
