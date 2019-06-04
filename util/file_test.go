package util

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsDir(t *testing.T) {
	f, err := ioutil.TempFile("", "testFile")
	assert.NoError(t, err)
	f.Close()
	defer os.Remove(f.Name())
	result := IsDir(f.Name())
	assert.Equal(t, false, result)

	dir, err := ioutil.TempDir("", "testDir")
	assert.NoError(t, err)
	defer os.Remove(dir)
	result = IsDir(dir)
	assert.Equal(t, true, result)
}

func TestGetHead(t *testing.T) {
	cmt, err := GetHead(".")
	assert.Error(t, err)
	assert.Empty(t, cmt)

	cmt, err = GetHead("../")
	assert.NoError(t, err)
	assert.NotEmpty(t, cmt)
}
