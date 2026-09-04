// This package only exists to define the struct that contains all CLI state.
package clistate

import "monorepo/twigg/commit"

type State struct {
	Current             commit.Commit
	ServerUrl           string
	ApiKey              string
	EnableUnsafeDevMode bool
}