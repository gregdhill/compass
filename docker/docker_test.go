package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	testURL = "https://auth.docker.io/token?service=registry.docker.io&scope=repository:hyperledger/burrow:pull"
)

func TestDockerAuth(t *testing.T) {
	var failUrls = []string{
		"www.example.com",
		"http://www.example.com",
		"https://www.example.com",
	}

	for _, url := range failUrls {
		token, err := GetAuthToken(url)
		assert.Error(t, err)
		assert.Empty(t, token)
	}

	token, err := GetAuthToken(testURL)
	assert.NoError(t, err)
	assert.NotEmpty(t, token)
}

func TestDockerHash(t *testing.T) {
	token, err := GetAuthToken(testURL)
	assert.NoError(t, err)
	assert.NotEmpty(t, token)

	result, err := GetImageHash("https://index.docker.io", "hyperledger/burrow", "latest", token)
	assert.NoError(t, err)
	assert.NotEmpty(t, result)

	auth, err := GetAuthToken(testURL)
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
