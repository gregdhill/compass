package core

import (
	"testing"

	"github.com/monax/compass/helm"
	"github.com/monax/compass/util"
	"github.com/stretchr/testify/assert"
	yaml "gopkg.in/yaml.v2"
)

func TestLinter(t *testing.T) {
	chart := newTestChart()

	charts := map[string]*Stage{"test": chart}
	pipeline := Pipeline{
		Stages: charts,
	}

	chart.Resource.(*helm.Chart).Namespace = ""
	pipeline.Lint(util.Values(map[string]string{"test_namespace": "somewhere-else"}))
	assert.Equal(t, "somewhere-else", pipeline.Stages["test"].Resource.(*helm.Chart).Namespace)
}

var testData = `
values:
  key: "value"

stages:
  test:
    kind: helm
    timeout: 2400
    name: chart
    repository: stable
    abandon: true
`

func TestUnmarshal(t *testing.T) {
	pipe := Pipeline{}
	err := yaml.Unmarshal([]byte(testData), &pipe)
	assert.NoError(t, err)
	assert.Equal(t, "chart", pipe.Stages["test"].Resource.(*helm.Chart).Name)
	assert.Equal(t, "stable", pipe.Stages["test"].Resource.(*helm.Chart).Repository)
}
