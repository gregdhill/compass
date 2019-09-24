package cmd

import (
	"fmt"

	"github.com/monax/relic"
	"github.com/spf13/cobra"
)

var commit string

var History relic.ImmutableHistory = relic.NewHistory("Compass", "https://github.com/monax/compass").
	MustDeclareReleases("", ``,
		"0.5.4 - 2019-09-24",
		`
		### Fixed
		- Initially generated values are now propagated
		`,

		"0.5.3 - 2019-09-23",
		`
		### Added
		- Builds can now take args
		`,

		"0.5.2 - 2019-09-23",
		`
		### Changed
		- Builds are now specified in the config
		- Tar packages build with target as tld
		`,

		"0.5.1 - 2019-08-27",
		`
		### Fixed
		- Ordering of build, tag, and deploy so that sha's properly populate
		`,

		"0.5.0 - 2019-08-16",
		`
		### Added
		- Sprig function library
		- Flag to output values in env var style

		### Fixed
		- JSON marshaling of values
		`,

		"0.4.3 - 2019-08-15",
		`
		### Changed
		- Unmarshal map[interface{}]interface{} on values
		`,

		"0.4.2 - 2019-06-12",
		`
		### Fixed
		- Only authenticate against required registry
		- Move main.go to root so "go install" works
		`,

		"0.4.1 - 2019-06-07",
		`
		### Fixed
		- Kubernetes deploys are now concurrent
		- Use correct status codes
		`,

		"0.4.0 - 2019-06-06",
		`
		### Changed
		- Separate output command
		
		### Added
		- Version command
		- Configurable kube / helm config
		`,
	)

func GetVersion(short bool) string {
	if commit != "" && !short {
		return fmt.Sprintf("v%s (%s)", History.CurrentVersion().String(), commit)
	}
	return fmt.Sprintf("v%s", History.CurrentVersion().String())
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number.",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(GetVersion(shortVersion))
	},
}
