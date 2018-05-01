package command

import (
	"io"
	"os"

	"golang.org/x/crypto/ssh/terminal"
)

// Execute adds all child commands to the root command and sets flags
// appropriately. This is called by main.main(). It only needs to happen once
// to the rootCmd.
func Execute(opts ...CommandOption) error {
	c, err := BuildConfig()
	if err != nil {
		return err
	}

	isTerminal := terminal.IsTerminal(int(os.Stdout.Fd()))
	var metaOpts []MetaOption
	if !isTerminal {
		metaOpts = append(metaOpts, WithMetaNoHeaders())
	}

	rootCmd := NewMeta(c)
	rootCmd.AddCommand(NewTail(c))

	for _, o := range opts {
		o(rootCmd)
	}
	return rootCmd.Execute()
}

type Command interface {
	SetOutput(io.Writer)
}

type CommandOption func(c Command)

func WithOutput(out io.Writer) CommandOption {
	return func(c Command) {
		c.SetOutput(out)
	}
}
