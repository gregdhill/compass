package core

import (
	"testing"

	"github.com/monax/compass/core/schema"
	"github.com/monax/compass/helm"
	"github.com/monax/compass/util"
	"github.com/stretchr/testify/assert"
	yaml "gopkg.in/yaml.v2"
)

func TestLinter(t *testing.T) {
	chart := newTestChart()
	charts := map[string]*schema.Stage{"test": chart}
	wf := &schema.Workflow{}
	wf.Stages = charts

	chart.Resource.(*helm.Chart).Namespace = ""
	Lint(wf, util.Values{"test.namespace": "somewhere-else"})
	assert.Equal(t, "somewhere-else", wf.Stages["test"].Resource.(*helm.Chart).Namespace)
}

var testData = `
stages:
  test:
    kind: helm
    timeout: 2400
    name: stable/chart
    forget: true
`

func TestUnmarshal(t *testing.T) {
	pipe := schema.Workflow{}
	err := yaml.Unmarshal([]byte(testData), &pipe)
	assert.NoError(t, err)
	assert.Equal(t, "stable/chart", pipe.Stages["test"].Resource.(*helm.Chart).Name)
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

func newTestWorkflow(names ...string) *schema.Workflow {
	wf := schema.NewWorkflow()
	for _, n := range names {
		wf.Stages[n] = newTestChart()
	}

	return wf
}

func TestRun(t *testing.T) {
	values := make(util.Values)
	t.Run("Workflows", func(t *testing.T) {
		t.Run("BasicRun", func(t *testing.T) {
			t.Parallel()
			workflow := newTestWorkflow("test1", "test2")
			workflow.Stages["test2"].Depends = []string{"test1"}
			err := Forward(workflow.Stages, values, false)
			assert.NoError(t, err)
		})
		t.Run("AdvancedRun", func(t *testing.T) {
			t.Parallel()
			workflow := newTestWorkflow("test1", "test2", "test3", "test4")
			workflow.Stages["test2"].Depends = []string{"test1"}
			workflow.Stages["test3"].Depends = []string{"test2"}
			err := Forward(workflow.Stages, values, false)
			assert.NoError(t, err)
		})
		t.Run("DepCycle", func(t *testing.T) {
			t.Parallel()
			workflow := newTestWorkflow("test1", "test2", "test3")
			workflow.Stages["test1"].Depends = []string{"test2"}
			workflow.Stages["test2"].Depends = []string{"test3"}
			workflow.Stages["test3"].Depends = []string{"test1"}
			err := Forward(workflow.Stages, values, false)
			assert.Error(t, err)
		})
	})
}
