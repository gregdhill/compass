package main

import (
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
)

func cleanInput(in string) (out string) {
	out = strings.Replace(in, "v2", "", -1)
	out = strings.Trim(out, "/")
	return
}

func removePattern(in, pattern string) string {
	return strings.Replace(in, pattern, "", 1)
}

func dockerHash(server, repo, tag, token string) string {
	server = cleanInput(server)
	repo = cleanInput(repo)
	tag = cleanInput(tag)

	client := &http.Client{}
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v2/%s/manifests/%s", server, repo, tag), nil)
	if err != nil {
		log.Fatalf("failed to get digest for %s:%s : %v\n", repo, tag, err)
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", os.Getenv(token)))
	auth := base64.StdEncoding.EncodeToString([]byte(os.Getenv(token)))
	req.Header.Add("Authorization", fmt.Sprintf("Basic %s", auth))
	req.Header.Add("Accept", "application/vnd.docker.distribution.manifest.v2+json")
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != 200 {
		log.Fatalf("failed to get digest for %s:%s : %v\n", repo, tag, err)
	}

	return strings.Split(resp.Header["Docker-Content-Digest"][0], ":")[1]
}
