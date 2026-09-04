// Package buildmeta provides metadata about the compiled binary.
//
// The exported variables are set at build time via -ldflags
// and can be used throughout the project to identify when and from
// which source the binary was built.
//
// Example usage:
//
//	go build -ldflags "\
//	  -X 'monorepo/buildmeta.Version=v1.0.0'"
package buildmeta

var (
	Version = "none"
)
