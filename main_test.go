package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompass(t *testing.T) {
	err := start([]string{"scroll"})
	assert.Errorf(t, err, "open scroll: no such file or directory")

	err = start([]string{"README.md"})
	assert.Error(t, err)
}
