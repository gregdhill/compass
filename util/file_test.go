package util

import (
	"archive/tar"
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
)

func TestIsDir(t *testing.T) {
	result := IsDir("thisdoesnotexist")
	assert.Equal(t, false, result)

	f, err := ioutil.TempFile("", "testFile")
	assert.NoError(t, err)
	f.Close()
	defer os.Remove(f.Name())
	result = IsDir(f.Name())
	assert.Equal(t, false, result)

	dir, err := ioutil.TempDir("", "testDir")
	assert.NoError(t, err)
	defer os.Remove(dir)
	result = IsDir(dir)
	assert.Equal(t, true, result)
}

func TestPackageDir(t *testing.T) {
	excluded := "file_test.go"
	included := "file.go"

	data, err := PackageDir(".", []string{excluded})
	assert.NoError(t, err)
	assert.NotNil(t, data)

	files := make([]string, 0)
	tr := tar.NewReader(bytes.NewBuffer(data))
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		files = append(files, hdr.Name)
	}
	assert.NotContains(t, files, excluded)
	assert.Contains(t, files, included)
}

func TestGetHead(t *testing.T) {
	cmt, err := GetHead(".")
	assert.Error(t, err)
	assert.Empty(t, cmt)

	cmt, err = GetHead("../")
	assert.NoError(t, err)
	assert.NotEmpty(t, cmt)
}

func TestRender(t *testing.T) {
	data, err := Render("", nil, template.FuncMap{})
	assert.NoError(t, err)
	assert.Nil(t, data)

	f, err := ioutil.TempFile("", "input.yaml")
	assert.NoError(t, err)
	_, err = f.WriteString("key: {{ getMe }}")
	assert.NoError(t, err)
	assert.NoError(t, f.Close())
	defer os.Remove(f.Name())

	data, err = Render(f.Name(), map[string]string{"key": "value"},
		template.FuncMap{"getMe": func() string {
			return "value"
		}})
	assert.NoError(t, err)
	assert.Equal(t, "key: value", string(data))
}
