package logcache

import (
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(UninstallCmd)
}

var UninstallCmd = &cobra.Command{
	Use: "CLI-MESSAGE-UNINSTALL",
	Run: func(cmd *cobra.Command, args []string) {},
}
