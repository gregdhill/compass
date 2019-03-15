package core

import (
	"testing"

	"github.com/monax/compass/helm"
	"github.com/stretchr/testify/assert"
)

func TestAppendVals(t *testing.T) {
	prev := Values(map[string]string{"test": "test"})
	next := Values(map[string]string{"test": "test"})
	prev.Append(next)
	assert.Equal(t, 1, len(prev))
}

func TestSliceVars(t *testing.T) {
	vals := Values(map[string]string{"dep1": "dep2"})
	actual := vals.ToSlice()
	assert.Equal(t, len(vals)*2, len(actual))
}

func TestLinter(t *testing.T) {
	c := Stage{
		Chart: helm.Chart{
			Name: "test",
		},
	}
	cs := map[string]*Stage{"Test": &c}
	p := Pipeline{
		Stages: cs,
	}

	p.Stages["Test"].Namespace = "test"
	p.Stages["Test"].Release = "test"
	testVals := Values(map[string]string{"Test_version": "1.1"})
	p.Lint(testVals)
	assert.Equal(t, "1.1", p.Stages["Test"].Version)

	p.Stages["Test"].Release = ""
	testVals = Values(map[string]string{"release": "test-release"})
	p.Lint(testVals)
	assert.Equal(t, "test-release-Test", p.Stages["Test"].Release)
}
