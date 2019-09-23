package schema

import (
	"fmt"

	"github.com/monax/compass/helm"
	"github.com/monax/compass/kube"
	"github.com/monax/compass/util"
)

type Image struct {
	Name      string `yaml:"name"`
	Context   string `yaml:"context"`
	Reference string `yaml:"reference"`
}

// Workflow represents the complete pipeline
type Workflow struct {
	Build  []Image           `yaml:"build"`
	Tag    []Image           `yaml:"tag"`
	Stages map[string]*Stage `yaml:"stages"`
	Values util.Values       `yaml:"values"`
}

func NewWorkflow() *Workflow {
	return &Workflow{
		Build:  make([]Image, 0),
		Tag:    make([]Image, 0),
		Stages: make(map[string]*Stage),
		Values: make(util.Values),
	}
}

// Jobs represent any shell scripts
type Jobs struct {
	Before []string `yaml:"before"`
	After  []string `yaml:"after"`
}

// Stage represents a single part of the deployment pipeline
type Stage struct {
	Actions `yaml:",inline"`
	Resource
}

type Actions struct {
	Depends  []string    `yaml:"depends"`  // dependencies
	Forget   bool        `yaml:"forget"`   // install only
	Template string      `yaml:"template"` // template file
	Jobs     Jobs        `yaml:"jobs"`     // bash jobs
	Kind     string      `yaml:"kind"`     // type of deploy
	Requires util.Values `yaml:"requires"` // env requirements
}

// Resource is the thing to be created / destroyed
type Resource interface {
	Lint(string, *util.Values) error
	Status() (bool, error)
	InstallOrUpgrade() error
	Delete() error
	Connect(interface{})
	SetInput([]byte)
	GetInput() []byte
}

// UnmarshalYAML allows us to determine the type of our resource
func (stg *Stage) UnmarshalYAML(unmarshal func(interface{}) error) error {
	act := new(Actions)
	if err := unmarshal(act); err != nil {
		return err
	}

	stg.Actions = *act
	switch act.Kind {
	case "kube", "kubernetes":
		var km kube.Manifest
		km.Timeout = 300
		if err := unmarshal(&km); err != nil {
			return err
		}
		stg.Resource = &km
	case "helm":
		var hc helm.Chart
		hc.Timeout = 300
		if err := unmarshal(&hc); err != nil {
			return err
		}
		stg.Resource = &hc
	default:
		return fmt.Errorf("kind '%s' unknown", act.Kind)
	}

	return nil
}
