package query

import (
	"strconv"
	"time"

	"github.com/jessevdk/go-flags"
)

type Opts struct {
	Query string

	Time time.Time

	Start time.Time
	End   time.Time
	Step  string
}

func NewOpts(args []string) (Opts, error) {
	var o Opts

	qa := &Params{}
	args, err := flags.ParseArgs(qa, args)
	if err != nil {
		return o, err
	}

	if len(args) < 1 {
		return o, ErrNoQuery
	}
	o.Query = args[0]

	if qa.Time != "" && (qa.Start != "" || qa.End != "" || qa.Step != "") {
		return o, ArgError{Arg: "-time", Msg: "cannot use flag along with -start, -end, or -step"}
	}

	o.Time, err = parseTimeOpt(qa.Time)
	if err != nil {
		return o, ArgError{Arg: "-time"}
	}

	o.Start, err = parseTimeOpt(qa.Start)
	if err != nil {
		return o, ArgError{Arg: "-start"}
	}

	o.End, err = parseTimeOpt(qa.End)
	if err != nil {
		return o, ArgError{Arg: "-end"}
	}

	o.Step = qa.Step

	return o, nil
}

func parseTimeOpt(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}

	t, err := time.Parse(time.RFC3339, s)
	if err == nil {
		return t, nil
	}

	i, err := strconv.Atoi(s)
	if err == nil {
		return time.Unix(int64(i), 0), nil
	}

	return time.Time{}, err
}
