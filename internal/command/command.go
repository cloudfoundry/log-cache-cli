// Package command implements various cf CLI plugin commands for communicating
// with Log Cache.
package command

import (
	"log"
	"os"
)

type sourceType string

const (
	_application sourceType = "application"
	_service     sourceType = "service"
	_platform    sourceType = "platform"
	_all         sourceType = "all"
	_default     sourceType = "default"
	_unknown     sourceType = "unknown"
)

func init() {
	log.SetOutput(os.Stdout)
	log.SetPrefix("")
	log.SetFlags(0)
}
