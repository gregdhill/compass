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
	pipeline := Stages(charts)

	chart.Resource.(*helm.Chart).Namespace = ""
	pipeline.Lint(util.Values{"test_namespace": "somewhere-else"})
	assert.Equal(t, "somewhere-else", pipeline["test"].Resource.(*helm.Chart).Namespace)
}

var testData = `
test:
    kind: helm
    timeout: 2400
    name: chart
    repository: stable
    forget: true
`

func TestUnmarshal(t *testing.T) {
	pipe := Stages{}
	err := yaml.Unmarshal([]byte(testData), &pipe)
	assert.NoError(t, err)
	assert.Equal(t, "chart", pipe["test"].Resource.(*helm.Chart).Name)
	assert.Equal(t, "stable", pipe["test"].Resource.(*helm.Chart).Repository)
}
