package main

import (
	"testing"

	"github.com/monax/compass/core"
	"github.com/monax/compass/core/helm"
	"github.com/stretchr/testify/assert"
)

func TestLinter(t *testing.T) {
	c := core.Stage{
		Chart: helm.Chart{
			Name: "test",
		},
	}
	cs := map[string]*core.Stage{"Test": &c}
	p := core.Pipeline{
		Stages: cs,
	}
	assert.Panics(t, func() { lint(&p, nil, "") })

	p.Stages["Test"].Namespace = "test"
	p.Stages["Test"].Release = "test"
	lint(&p, map[string]string{"Test_version": "1.1"}, "")
	assert.Equal(t, "1.1", p.Stages["Test"].Version)

	p.Stages["Test"].Release = ""
	lint(&p, map[string]string{"release": "test-release"}, "")
	assert.Equal(t, "test-release-test", p.Stages["Test"].Release)

	p.Stages["Test"].Release = ""
	lint(&p, map[string]string{"Test_release": "test-release"}, "")
	assert.Equal(t, "test-release-test", p.Stages["Test"].Release)
}
