package platform

type Source struct {
	GUID string `json:"guid"`
	Name string `json:"name"`
	Type SourceType
}

func NewSource(GUID, name string, st SourceType) Source {
	return Source{
		GUID: GUID,
		Name: name,
		Type: st,
	}
}

type SourceInfo struct {
	Resources []Source `json:"resources"`
}
