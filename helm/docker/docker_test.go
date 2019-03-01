package docker

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDockerHash(t *testing.T) {
	url := "'https://auth.docker.io/token?service=registry.docker.io&scope=repository:hyperledger/burrow:pull'"
	url = cleanInput(url)
	resp, err := http.Get(url)
	assert.NoError(t, err)
	body, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	assert.NoError(t, err)
	jsonToken := make(map[string]interface{})
	err = json.Unmarshal(body, &jsonToken)
	assert.NoError(t, err)

	result, err := GetImageHash("https://index.docker.io", "hyperledger/burrow", "latest", jsonToken["token"].(string))
	assert.NoError(t, err)
	assert.NotEmpty(t, result)

	auth, err := GetAuthToken(url)
	assert.NoError(t, err)
	result, err = GetImageHash("https://index.docker.io", "hyperledger/burrow", "latest", auth)
	assert.NoError(t, err)
	assert.NotEmpty(t, result)
}

func TestCleanInput(t *testing.T) {
	assert.Equal(t, "https://index.docker.io", cleanInput("https://index.docker.io/"))
	assert.Equal(t, "https://index.docker.io", cleanInput("https://index.docker.io/v2"))
	assert.Equal(t, "https://index.docker.io", cleanInput("https://index.docker.io/v2/"))
	assert.Equal(t, "repo/image", cleanInput("/repo/image/"))
	assert.Equal(t, "latest", cleanInput("/latest/"))
}
