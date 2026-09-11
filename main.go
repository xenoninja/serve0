package main

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/xenoninja/serve0/internal/app"
)

// version is set by the release builder; go install uses module build metadata.
var version string

func buildVersion() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if info.Main.Version != "" && info.Main.Version != "(devel)" {
			return info.Main.Version
		}
		revision := ""
		dirty := false
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				revision = setting.Value
			}
			if setting.Key == "vcs.modified" {
				dirty = setting.Value == "true"
			}
		}
		if revision != "" {
			if dirty {
				revision += "-dirty"
			}
			return "devel+" + revision
		}
	}
	return "devel"
}

func main() {
	if err := app.Run(os.Args[1:], buildVersion()); err != nil {
		fmt.Fprintln(os.Stderr, "serve0:", err)
		os.Exit(1)
	}
}
