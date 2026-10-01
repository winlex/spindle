package buildinfo

import (
	"fmt"
	"runtime"
	"time"
)

// значение подменяется линкером через `-ldflags -X` при сборке релиза
var version = "dev"

type Info struct {
	Name      string
	Version   string
	GoVersion string
	StartedAt time.Time
}

func New() Info {
	return Info{
		Name:      "spindle-api",
		Version:   version,
		GoVersion: runtime.Version(),
		StartedAt: time.Now().UTC(),
	}
}

func (i Info) String() string {
	return fmt.Sprintf("%s %s (go %s, started %s)",
		i.Name, i.Version, i.GoVersion, i.StartedAt.Format(time.RFC3339))
}
