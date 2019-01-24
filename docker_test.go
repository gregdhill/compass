package main

import (
	"testing"

	"gotest.tools/assert"
)

func TestDockerHash(t *testing.T) {
	assert.Equal(t, "latest", DockerHash("https://index.docker.io", "hyperledger/burrow", "latest", "DOCKER_TOKEN"))
}

func TestCleanInput(t *testing.T) {
	assert.Equal(t, "https://index.docker.io", cleanToken("https://index.docker.io/"))
	assert.Equal(t, "https://index.docker.io", cleanToken("https://index.docker.io/v2"))
	assert.Equal(t, "https://index.docker.io", cleanToken("https://index.docker.io/v2/"))
	assert.Equal(t, "repo/image", cleanToken("/repo/image/"))
	assert.Equal(t, "latest", cleanToken("/latest/"))
}
