package schema

import (
	"fmt"

	"github.com/mitchellh/mapstructure"
	"github.com/monax/compass/helm"
	"github.com/monax/compass/kube"
	"github.com/monax/compass/util"
)

// Workflow represents the complete pipeline
type Workflow struct {
	Build  map[string]string `yaml:"build"`
	Tag    map[string]string `yaml:"tag"`
	Stages map[string]*Stage `yaml:"stages"`
	Values util.Values       `yaml:"values"`
}

func NewWorkflow() *Workflow {
	return &Workflow{
		Build:  make(map[string]string),
		Tag:    make(map[string]string),
		Stages: make(map[string]*Stage),
	}
}

// Jobs represent any shell scripts
type Jobs struct {
	Before []string `yaml:"before"`
	After  []string `yaml:"after"`
}

// Stage represents a single part of the deployment pipeline
type Stage struct {
	Depends  []string    `yaml:"depends"`  // dependencies
	Forget   bool        `yaml:"forget"`   // install only
	Template string      `yaml:"template"` // template file
	Jobs     Jobs        `yaml:"jobs"`     // bash jobs
	Kind     string      `yaml:"kind"`     // type of deploy
	Requires util.Values `yaml:"requires"` // env requirements
	Values   interface{} `yaml:"values"`   // yaml values
	Resource
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
	this := make(map[string]interface{}, 0)
	if err := unmarshal(&this); err != nil {
		return err
	}

	if err := mapstructure.Decode(this, stg); err != nil {
		return err
	}

	switch this["kind"] {
	case "kube", "kubernetes":
		var km kube.Manifest
		km.Timeout = 300
		if err := mapstructure.Decode(this, &km); err != nil {
			return err
		}
		stg.Resource = &km
	case "helm":
		var hc helm.Chart
		hc.Timeout = 300
		if err := mapstructure.Decode(this, &hc); err != nil {
			return err
		}
		stg.Resource = &hc
	default:
		return fmt.Errorf("kind '%s' unknown", this["kind"])
	}

	return nil
}
