// +build integration

package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDockerHash(t *testing.T) {
	result, err := GetImageHash("index.docker.io/hyperledger/burrow:latest")
	assert.NoError(t, err)
	assert.NotEmpty(t, result)

	result, err = GetImageHash("quay.io/monax/burrow:latest")
	assert.NoError(t, err)
	assert.NotEmpty(t, result)
}
