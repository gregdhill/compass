package docker

import (
	"os"
	"strings"

	"github.com/docker/cli/cli/config"
	"github.com/docker/docker/api/types"
	"github.com/genuinetools/reg/registry"
)

func getAuth(ref string) (types.AuthConfig, error) {
	server := strings.Split(ref, "/")[0]
	acs, err := config.LoadDefaultConfigFile(os.Stderr).GetAllCredentials()
	if err != nil {
		return types.AuthConfig{}, err
	}

	return types.AuthConfig(acs[server]), nil
}

func getDigest(ref string, conf types.AuthConfig) (string, error) {
	server := strings.Split(ref, "/")[0]
	reg, err := registry.New(conf, registry.Opt{
		Domain:   server,
		SkipPing: true, // otherwise this is slow
	})
	if err != nil {
		return "", err
	}

	img, err := registry.ParseImage(ref)
	if err != nil {
		return "", err
	}

	dig, err := reg.Digest(img)
	if err != nil {
		return "", err
	}

	return dig.Encoded(), nil
}

// GetImageHash fetches the latest image digest for the given ref (server/app:tag)
func GetImageHash(ref string) (string, error) {
	if err := checkTag(ref); err != nil {
		return "", err
	}

	authConfig, err := getAuth(ref)
	if err != nil {
		authConfig = types.AuthConfig{}
	}

	return getDigest(ref, authConfig)
}
