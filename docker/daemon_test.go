package docker

import (
	"context"
	"io"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/registry"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

const (
	testRefWithTag = "quay.io/repo/app:tag"
	testRefNoTag   = "quay.io/repo/app"
)

func TestTagging(t *testing.T) {
	err := checkTag(testRefWithTag)
	assert.NoError(t, err)

	err = checkTag(testRefNoTag)
	assert.Error(t, err)

	actual, err := tagRef(testRefWithTag, ".")
	assert.NoError(t, err)
	assert.Equal(t, testRefWithTag, actual)

	actual, err = tagRef(testRefNoTag, ".")
	assert.Error(t, err)
	actual, err = tagRef(testRefNoTag, "../")
	assert.NoError(t, err)
}

type imgCli struct {
}

func (ic imgCli) ImageBuild(ctx context.Context, context io.Reader, options types.ImageBuildOptions) (types.ImageBuildResponse, error) {
	body := ioutil.NopCloser(strings.NewReader("{\"stream\": \"building image\"}"))
	return types.ImageBuildResponse{
		Body: body,
	}, nil
}
func (ic imgCli) ImageCreate(ctx context.Context, parentReference string, options types.ImageCreateOptions) (io.ReadCloser, error) {
	return nil, nil
}
func (ic imgCli) ImageHistory(ctx context.Context, image string) ([]types.ImageHistory, error) {
	return nil, nil
}
func (ic imgCli) ImageImport(ctx context.Context, source types.ImageImportSource, ref string, options types.ImageImportOptions) (io.ReadCloser, error) {
	return nil, nil
}
func (ic imgCli) ImageInspectWithRaw(ctx context.Context, image string) (types.ImageInspect, []byte, error) {
	return types.ImageInspect{}, nil, nil
}
func (ic imgCli) ImageList(ctx context.Context, options types.ImageListOptions) ([]types.ImageSummary, error) {
	return nil, nil
}
func (ic imgCli) ImageLoad(ctx context.Context, input io.Reader, quiet bool) (types.ImageLoadResponse, error) {
	return types.ImageLoadResponse{}, nil
}
func (ic imgCli) ImagePull(ctx context.Context, ref string, options types.ImagePullOptions) (io.ReadCloser, error) {
	return nil, nil
}
func (ic imgCli) ImagePush(ctx context.Context, ref string, options types.ImagePushOptions) (io.ReadCloser, error) {
	if ref != testRefWithTag {
		return nil, errors.New("ref incorrect")
	}
	return ioutil.NopCloser(strings.NewReader("{\"status\": \"downloaded image\"}")), nil
}
func (ic imgCli) ImageRemove(ctx context.Context, image string, options types.ImageRemoveOptions) ([]types.ImageDelete, error) {
	return nil, nil
}
func (ic imgCli) ImageSearch(ctx context.Context, term string, options types.ImageSearchOptions) ([]registry.SearchResult, error) {
	return nil, nil
}
func (ic imgCli) ImageSave(ctx context.Context, images []string) (io.ReadCloser, error) {
	return nil, nil
}
func (ic imgCli) ImageTag(ctx context.Context, image, ref string) error {
	return nil
}
func (ic imgCli) ImagesPrune(ctx context.Context, pruneFilter filters.Args) (types.ImagesPruneReport, error) {
	return types.ImagesPruneReport{}, nil
}

func TestBuildPush(t *testing.T) {
	cli := imgCli{}
	err := buildImage(context.TODO(), cli, "../", testRefWithTag)
	assert.NoError(t, err)

	err = pushImage(context.TODO(), cli, "", testRefWithTag)
	assert.NoError(t, err)
}
