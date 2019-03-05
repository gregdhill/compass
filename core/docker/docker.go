package docker

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

func cleanInput(in string) (out string) {
	out = strings.Replace(in, "v2", "", -1)
	out = strings.Trim(out, "/")
	if (out[0] == '\'' && out[len(out)-1] == '\'') ||
		(out[0] == '"' && out[len(out)-1] == '"') {
		out = out[1 : len(out)-1]
	}
	return
}

// GetAuthToken fetches the docker api auth url and returns a valid token
func GetAuthToken(url string) (token string, err error) {
	resp, err := http.Get(url)
	if err != nil {
		return token, fmt.Errorf("failed to get url %s : %v", url, err)
	}
	body, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		return token, fmt.Errorf("failed to read response body: %v", err)
	}
	auth := make(map[string]interface{})
	err = json.Unmarshal(body, &auth)
	if err != nil {
		return token, fmt.Errorf("failed to unmarshal response body: %v", err)
	}
	return auth["token"].(string), nil
}

// GetImageHash fetches the latest image digest for the given tag
func GetImageHash(server, repo, tag, token string) (digest string, err error) {
	server = cleanInput(server)
	repo = cleanInput(repo)
	tag = cleanInput(tag)

	client := &http.Client{}
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v2/%s/manifests/%s", server, repo, tag), nil)
	if err != nil {
		return digest, fmt.Errorf("failed to get digest for %s:%s : %v", repo, tag, err)
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
	auth := base64.StdEncoding.EncodeToString([]byte(token))
	req.Header.Add("Authorization", fmt.Sprintf("Basic %s", auth))
	req.Header.Add("Accept", "application/vnd.docker.distribution.manifest.v2+json")
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != 200 {
		return digest, fmt.Errorf("failed to get digest for %s:%s : %v", repo, tag, err)
	}

	return strings.Split(resp.Header["Docker-Content-Digest"][0], ":")[1], nil
}
