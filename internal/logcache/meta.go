package logcache

import (
	"fmt"

	"code.cloudfoundry.org/cli/plugin"
	"github.com/spf13/cobra"
)

var yolo string

func init() {
	MetaCmd.Flags().StringVarP(&yolo, "source", "s", "", "Source directory to read from")
	rootCmd.AddCommand(MetaCmd)
}

var MetaCmd = &cobra.Command{
	Use: "log-meta",
	Run: func(cmd *cobra.Command, args []string) {
		conn := cmd.Context().Value("conn").(plugin.CliConnection)
		fmt.Println("testing")
		fmt.Println(args)
		fmt.Println(yolo)
		u, err := conn.Username()
		fmt.Println("check", u, "or error", err)
	},
}
