package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetDigest(t *testing.T) {
	result, err := GetImageDigest("index.docker.io/hyperledger/burrow:latest")
	assert.NoError(t, err)
	assert.NotEmpty(t, result)
}
