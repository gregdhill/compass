package project

import (
	"fmt"

	"github.com/monax/relic"
)

var commit string

var History relic.ImmutableHistory = relic.NewHistory("Compass", "https://github.com/monax/compass").
	MustDeclareReleases("", ``,
		"0.4.0 - 2019-06-06",
		`
		### Changed
		- Separate output command
		
		### Added
		- Version command
		- Configurable kube / helm config
		`,
	)

func GetVersion() string {
	if commit != "" {
		return fmt.Sprintf("%s (%s)", History.CurrentVersion().String(), commit)
	}
	return History.CurrentVersion().String()
}
