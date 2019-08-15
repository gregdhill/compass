package cmd

import (
	"fmt"

	"github.com/monax/relic"
	"github.com/spf13/cobra"
)

var commit string

var History relic.ImmutableHistory = relic.NewHistory("Compass", "https://github.com/monax/compass").
	MustDeclareReleases("", ``,
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
