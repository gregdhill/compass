package util

import (
	"archive/tar"
	"bytes"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"gopkg.in/src-d/go-git.v4"
)

// IsDir returns true if the given path corresponds to a directory
func IsDir(name string) bool {
	file, err := os.Open(name)
	if err != nil {
		return false
	}
	stat, err := file.Stat()
	if err != nil {
		return false
	}
	return stat.IsDir()
}

// PackageDir recursively adds files under the given directory to an in-memory tar file
func PackageDir(dir string, ignore []string) ([]byte, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	defer tw.Close()

	if err := filepath.Walk(dir, func(url string, file os.FileInfo, err error) error {
		hdr := &tar.Header{
			// we want the given dir to be the top level
			Name: strings.TrimLeft(url, path.Base(dir)),
			Mode: 0600,
			Size: file.Size(),
		}
		if file.IsDir() {
			hdr.Typeflag = tar.TypeDir
		}

		// .dockerignore
		for _, i := range ignore {
			if match, _ := regexp.MatchString(i, url); match {
				return nil
			}
		}

		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}

		contents, err := ioutil.ReadFile(url)
		if _, err := tw.Write(contents); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// GetHead returns the commit ID of the repo located at the given path
func GetHead(path string) (string, error) {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return "", err
	}

	ref, err := repo.Head()
	if err != nil {
		return "", err
	}

	return ref.Hash().String(), nil
}

// RenderFile reads a file and templates it according to the provided functions
func RenderFile(name string, v Values, funcs template.FuncMap) ([]byte, error) {
	if name == "" {
		return nil, nil
	}

	data, err := ioutil.ReadFile(name)
	if err != nil {
		return nil, err
	}

	return Render(name, data, v, funcs)
}

func Render(name string, data []byte, v Values, funcs template.FuncMap) ([]byte, error) {
	buf := new(bytes.Buffer)
	t, err := template.New(name).Funcs(funcs).Parse(string(data))
	if err != nil {
		return nil, err
	}

	err = t.Execute(buf, v)
	return buf.Bytes(), err
}
