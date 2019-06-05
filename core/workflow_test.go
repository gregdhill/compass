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
    name: stable/chart
    forget: true
`

func TestUnmarshal(t *testing.T) {
	pipe := Stages{}
	err := yaml.Unmarshal([]byte(testData), &pipe)
	assert.NoError(t, err)
	assert.Equal(t, "stable/chart", pipe["test"].Resource.(*helm.Chart).Name)
}

func TestDepends(t *testing.T) {
	var bicycle = []struct {
		depends  Depends
		expected bool
	}{
		// acyclic
		{Depends{"test1": &Node{}}, false},
		{Depends{"test1": &Node{}, "test2": &Node{Edges: []string{"test1"}}}, false},

		// cyclic
		{Depends{"test1": &Node{Edges: []string{"test1"}}}, true},
		{Depends{"test1": &Node{Edges: []string{"test3"}},
			"test2": &Node{Edges: []string{"test1"}},
			"test3": &Node{Edges: []string{"test2"}}}, true},
	}

	for _, tt := range bicycle {
		actual := tt.depends.IsCyclic()
		if actual != tt.expected {
			t.Errorf("expected %t, actual %t", tt.expected, actual)
		}
	}
}

func newTestWorkflow(names ...string) *Stages {
	stages := make(Stages, len(names))
	for _, n := range names {
		stages[n] = newTestChart()
	}

	return &stages
}

func TestRun(t *testing.T) {
	values := make(util.Values)
	t.Run("Workflows", func(t *testing.T) {
		t.Run("BasicRun", func(t *testing.T) {
			t.Parallel()
			workflow := newTestWorkflow("test1", "test2")
			(*workflow)["test2"].Depends = []string{"test1"}
			err := workflow.Run(values, false)
			assert.NoError(t, err)
		})
		t.Run("AdvancedRun", func(t *testing.T) {
			t.Parallel()
			workflow := newTestWorkflow("test1", "test2", "test3", "test4")
			(*workflow)["test2"].Depends = []string{"test1"}
			(*workflow)["test3"].Depends = []string{"test2"}
			err := workflow.Run(values, false)
			assert.NoError(t, err)
		})
		t.Run("DepCycle", func(t *testing.T) {
			t.Parallel()
			workflow := newTestWorkflow("test1", "test2", "test3")
			(*workflow)["test1"].Depends = []string{"test2"}
			(*workflow)["test2"].Depends = []string{"test3"}
			(*workflow)["test3"].Depends = []string{"test1"}
			err := workflow.Run(values, false)
			assert.Error(t, err)
		})
	})
}
