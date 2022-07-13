package query

import (
	"errors"
	"fmt"
)

var ErrNoQuery = errors.New("PROMQL_QUERY must be provided")

type ArgError struct {
	Arg string
	Msg string
}

func (e ArgError) Error() string {
	if e.Msg != "" {
		return fmt.Sprintf("%s: %s", e.Arg, e.Msg)
	}
	return fmt.Sprintf("%s: invalid format", e.Arg)
}

type ClientError struct {
	Msg string
}

func (e ClientError) Error() string {
	return fmt.Sprintf("log cache client error: %s", e.Msg)
}

type RequestError struct {
	Type string
	Msg  string
}

func (e RequestError) Error() string {
	return fmt.Sprintf("log cache error: %s: %s", e.Type, e.Msg)
}

type MarshalError struct {
	Msg string
}

func (e MarshalError) Error() string {
	return fmt.Sprintf("response body unmarshalling error: %s", e.Msg)
}
