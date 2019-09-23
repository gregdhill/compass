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
	"github.com/monax/compass/core/schema"
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

func buildImage(ctx context.Context, cli client.ImageAPIClient, img schema.Image) error {
	log.Infof("Packaging from: %s", img.Context)
	ignore, err := readIgnore(path.Join(img.Context, ".dockerignore"))
	if err != nil {
		return err
	}

	tarArch, err := util.PackageDir(img.Context, ignore)
	if err != nil {
		return err
	}

	fmt.Println(img.Args)

	log.Info("Sending context to daemon...")
	imageBuildResponse, err := cli.ImageBuild(
		ctx,
		bytes.NewReader(tarArch),
		types.ImageBuildOptions{
			BuildArgs:  img.Args,
			Dockerfile: "Dockerfile",
			Tags:       []string{img.Reference},
		})
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("cannot build docker image with ref %s", img.Reference))
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

func checkAndFixRef(ref string) (string, error) {
	if imgParts := strings.Split(ref, "/"); len(imgParts) > 2 {
		// server is identified
		return ref, nil
	} else if len(imgParts) == 2 {
		// guessing it's a dockerhub reference
		return fmt.Sprintf("%s/%s", DockerHub, ref), nil
	}
	return ref, fmt.Errorf("image ref '%s' not valid", ref)
}

func checkTag(ref string) error {
	if imgTag := strings.Split(ref, ":"); len(imgTag) > 1 {
		// already tagged
		return nil
	}
	return errors.New("no image tag supplied")
}

func tagRef(ref string) (string, error) {
	if err := checkTag(ref); err == nil {
		return ref, nil
	}
	commit, err := util.GetHead(".")
	if err != nil {
		return "", err
	}
	log.Infof("No tag supplied, using last commit id: %s", commit)
	return fmt.Sprintf("%s:%s", ref, commit), nil
}

// BuildAndPush constructs a local image and commits it to the remote repository
func BuildAndPush(ctx context.Context, img schema.Image) (string, error) {
	cli, err := client.NewEnvClient()
	if err != nil {
		return "", err
	}

	if img.Reference, err = tagRef(img.Reference); err != nil {
		return "", err
	}

	if err := buildImage(ctx, cli, img); err != nil {
		return "", err
	}

	authConfig, err := getAuth(img.Reference)
	if err != nil {
		return "", err
	}

	authToken, err := serializeAuth(authConfig)
	if err != nil {
		return "", err
	}

	if err := pushImage(ctx, cli, authToken, img.Reference); err != nil {
		return "", err
	}

	return getDigest(img.Reference, authConfig)
}
