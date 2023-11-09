package logcache

import (
	"fmt"

	"code.cloudfoundry.org/cli/plugin"
)

func init() {
	AddCommand(&MetaCmd{})
}

type MetaCmd struct{}

func (mc *MetaCmd) Run(conn plugin.CliConnection, args []string) {
	fmt.Println("yup")
}

func (mc *MetaCmd) Metadata() plugin.Command {
	return plugin.Command{
		Name: "log-meta",
	}
}
