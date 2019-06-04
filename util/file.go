package util

import (
	"archive/tar"
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"

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
func PackageDir(path string) ([]byte, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	defer tw.Close()

	if err := filepath.Walk(path, func(path string, file os.FileInfo, err error) error {
		hdr := &tar.Header{
			Name: path,
			Mode: 0600,
			Size: file.Size(),
		}
		if file.IsDir() {
			hdr.Typeflag = tar.TypeDir
		}

		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}

		contents, err := ioutil.ReadFile(path)
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
