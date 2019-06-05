package docker

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/moby/moby/client"
	"github.com/monax/compass/util"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

func streamLogs(body io.ReadCloser, field string) error {
	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		line := make(map[string]string)
		json.Unmarshal(scanner.Bytes(), &line)
		out := strings.TrimSuffix(line[field], "\n")
		if out != "" {
			log.Info(out)
		}
	}

	return scanner.Err()
}

func readIgnore(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		// if file doesn't exist
		// just package everything
		return nil, nil
	}
	defer file.Close()

	out := make([]string, 0)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		out = append(out, scanner.Text())
	}

	return out, scanner.Err()
}

func buildImage(ctx context.Context, cli client.ImageAPIClient, buildCtx, ref string) error {
	log.Infof("Packaging from: %s", buildCtx)

	ignore, err := readIgnore(path.Join(buildCtx, ".dockerignore"))
	if err != nil {
		return err
	}

	tarArch, err := util.PackageDir(buildCtx, ignore)
	if err != nil {
		return err
	}

	imageBuildResponse, err := cli.ImageBuild(
		ctx,
		bytes.NewReader(tarArch),
		types.ImageBuildOptions{
			Dockerfile: path.Join(buildCtx, "Dockerfile"),
			Tags:       []string{ref},
		})
	if err != nil {
		return errors.Wrap(err, "cannot build docker image")
	}
	defer imageBuildResponse.Body.Close()

	return streamLogs(imageBuildResponse.Body, "stream")
}

func pushImage(ctx context.Context, cli client.ImageAPIClient, auth, ref string) error {
	imagePushResp, err := cli.ImagePush(ctx, ref,
		types.ImagePushOptions{
			RegistryAuth: auth,
		})
	if err != nil {
		return err
	}
	defer imagePushResp.Close()

	return streamLogs(imagePushResp, "status")
}

func serializeAuth(authConfig types.AuthConfig) (string, error) {
	authToken, err := json.Marshal(authConfig)
	if err != nil {
		return "", err
	}

	return base64.URLEncoding.EncodeToString(authToken), nil
}

func checkTag(ref string) error {
	if imgTag := strings.Split(ref, ":"); len(imgTag) > 1 {
		// already tagged
		return nil
	}
	return errors.New("no image tag supplied")
}

func tagRef(ref, buildCtx string) (string, error) {
	if err := checkTag(ref); err == nil {
		return ref, nil
	}
	commit, err := util.GetHead(buildCtx)
	if err != nil {
		return "", err
	}
	log.Infof("No tag supplied, using last commit id: %s", commit)
	return fmt.Sprintf("%s:%s", ref, commit), nil
}

// BuildAndPush constructs a local image and commits it to the remote repository
func BuildAndPush(ctx context.Context, buildCtx, ref string) (string, error) {
	cli, err := client.NewEnvClient()
	if err != nil {
		return "", err
	}

	if ref, err = tagRef(ref, buildCtx); err != nil {
		return "", err
	}

	if err := buildImage(ctx, cli, buildCtx, ref); err != nil {
		return "", err
	}

	authConfig, err := getAuth(ref)
	if err != nil {
		return "", err
	}

	authToken, err := serializeAuth(authConfig)
	if err != nil {
		return "", err
	}

	if err := pushImage(ctx, cli, authToken, ref); err != nil {
		return "", err
	}

	return getDigest(ref, authConfig)
}
