package main

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"strings"
)

func cleanToken(in string) (out string) {
	out = strings.Replace(in, "v2", "", -1)
	out = strings.Trim(out, "/")
	return out
}

func removePattern(in, pattern string) string {
	return strings.Replace(in, pattern, "", 1)
}

func dockerHash(server, repo, tag, token string) string {
	server = cleanToken(server)
	repo = cleanToken(repo)
	tag = cleanToken(tag)

	client := &http.Client{}
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v2/%s/manifests/%s", server, repo, tag), nil)
	if err != nil {
		return tag
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", os.Getenv(token)))
	auth := base64.StdEncoding.EncodeToString([]byte(os.Getenv(token)))
	req.Header.Add("Authorization", fmt.Sprintf("Basic %s", auth))
	req.Header.Add("Accept", "application/vnd.docker.distribution.manifest.v2+json")
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != 200 {
		return tag
	}

	return strings.Split(resp.Header["Docker-Content-Digest"][0], ":")[1]
}
