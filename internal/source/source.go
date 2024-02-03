package source

import (
	"strings"

	"code.cloudfoundry.org/cli/plugin"
)

type Type string

const (
	ApplicationType = "application"
	ServiceType     = "service"
	UnknownType     = "unknown"
)

type Source struct {
	ID   string
	Name string
	Type Type
}

func Get(conn plugin.CliConnection, sourceID string) Source {
	if s, err := isApp(conn, sourceID); err == nil {
		return s
	}
	if s, err := isService(conn, sourceID); err == nil {
		return s
	}
	return Source{ID: sourceID, Type: UnknownType}
}

func isApp(conn plugin.CliConnection, name string) (Source, error) {
	result, err := conn.CliCommandWithoutTerminalOutput("app", name, "--guid")
	if err != nil {
		return Source{}, err
	}
	guid := strings.Join(result, "")
	return Source{ID: guid, Type: ApplicationType, Name: name}, nil
}

func isService(conn plugin.CliConnection, name string) (Source, error) {
	result, err := conn.CliCommandWithoutTerminalOutput("service", name, "--guid")
	if err != nil {
		return Source{}, err
	}
	guid := strings.Join(result, "")
	return Source{ID: guid, Type: ServiceType, Name: name}, nil
}
