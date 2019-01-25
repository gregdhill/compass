package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDockerHash(t *testing.T) {
	assert.Equal(t, "latest", dockerHash("https://index.docker.io", "hyperledger/burrow", "latest", "DOCKER_TOKEN"))
}

func TestCleanInput(t *testing.T) {
	assert.Equal(t, "https://index.docker.io", cleanToken("https://index.docker.io/"))
	assert.Equal(t, "https://index.docker.io", cleanToken("https://index.docker.io/v2"))
	assert.Equal(t, "https://index.docker.io", cleanToken("https://index.docker.io/v2/"))
	assert.Equal(t, "repo/image", cleanToken("/repo/image/"))
	assert.Equal(t, "latest", cleanToken("/latest/"))
}

func TestRemovePattern(t *testing.T) {
	assert.Equal(t, "index.docker.io", removePattern("https://index.docker.io", "https://"))
}
