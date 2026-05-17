package version

import (
	"fmt"
	"runtime"
)

var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"build_time"`
	GoVersion string `json:"go_version"`
}

func Get() Info {
	return Info{
		Version:   Version,
		Commit:    Commit,
		BuildTime: BuildTime,
		GoVersion: runtime.Version(),
	}
}

func String() string {
	return fmt.Sprintf("music-online version %s built on %s commit=%s go=%s",
		Version, BuildTime, Commit, runtime.Version())
}
