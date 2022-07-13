package query

type Params struct {
	Time string `long:"time"`

	Start string `long:"start"`
	End   string `long:"end"`
	Step  string `long:"step"`
}
