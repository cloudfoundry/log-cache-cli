package platform

type ServiceInstance struct {
	Metadata struct {
		GUID string `json:"guid"`
	} `json:"metadata"`
	Entity struct {
		Name string `json:"name"`
	} `json:"entity"`
}

type ServicesResponse struct {
	Resources []ServiceInstance `json:"resources"`
}
