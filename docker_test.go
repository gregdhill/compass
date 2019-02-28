package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDockerHash(t *testing.T) {
	resp, err := http.Get("https://auth.docker.io/token?service=registry.docker.io&scope=repository:hyperledger/burrow:pull")
	assert.NoError(t, err)
	body, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	assert.NoError(t, err)
	jsonToken := make(map[string]interface{})
	err = json.Unmarshal(body, &jsonToken)
	assert.NoError(t, err)

	os.Setenv("DOCKER_TOKEN", jsonToken["token"].(string))
	result := dockerHash("https://index.docker.io", "hyperledger/burrow", "latest", "DOCKER_TOKEN")
	assert.NotEmpty(t, result)
}

func TestCleanInput(t *testing.T) {
	assert.Equal(t, "https://index.docker.io", cleanInput("https://index.docker.io/"))
	assert.Equal(t, "https://index.docker.io", cleanInput("https://index.docker.io/v2"))
	assert.Equal(t, "https://index.docker.io", cleanInput("https://index.docker.io/v2/"))
	assert.Equal(t, "repo/image", cleanInput("/repo/image/"))
	assert.Equal(t, "latest", cleanInput("/latest/"))
}

func TestRemovePattern(t *testing.T) {
	assert.Equal(t, "index.docker.io", removePattern("https://index.docker.io", "https://"))
}
