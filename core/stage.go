package core

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/monax/compass/helm"
	"github.com/monax/compass/kube"
	"github.com/monax/compass/util"
	log "github.com/sirupsen/logrus"
)

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

// Resource is the thing to be created / destroyed
type Resource interface {
	Lint(string, *util.Values) error
	Status() (bool, error)
	Install() error
	Upgrade() error
	Delete() error
	Connect(interface{})
	SetInput([]byte)
	GetInput() []byte
}

func shellJobs(values []string, jobs []string) {
	for _, command := range jobs {
		log.Infof("running job: %s\n", command)
		args := strings.Fields(command)
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Env = append(values, os.Environ()...)
		stdout, err := cmd.Output()
		if stdout != nil {
			fmt.Println(string(stdout))
		}
		if err != nil {
			panic(fmt.Errorf("job '%s' exited with error: %v", command, err))
		}
	}
}

func checkRequires(values util.Values, reqs util.Values) error {
	for k, v := range reqs {
		if _, exists := values[k]; !exists {
			return fmt.Errorf("argument '%s' not given", k)
		} else if values[k] != v {
			return fmt.Errorf("argument '%s' not given value '%s'", k, v)
		}
	}
	return nil
}

// Destroy removes resource
func (stg *Stage) Destroy(logger *log.Entry, key string, global util.Values, force bool) error {
	// only continue if required variables are set
	if err := checkRequires(global, stg.Requires); err != nil {
		logger.Infof("Ignoring: %s: %s", key, err.Error())
		return nil
	}

	// don't delete by default
	if !force && stg.Forget {
		logger.Infof("Ignoring: %s", key)
		return fmt.Errorf("Not deleting stage %s", key)
	}

	logger.Infof("Deleting: %s", key)

	return stg.Delete()
}

// Create installs / upgrades resource
func (stg *Stage) Create(logger *log.Entry, key string, global util.Values, force bool) error {
	// stop if already installed and abandoned
	installed, _ := stg.Status()
	if installed && !force && stg.Forget {
		logger.Infof("Ignoring: %s", key)
		return nil
	}

	local := global.Duplicate()
	shellVars := local.ToSlice()
	if err := checkRequires(local, stg.Requires); err != nil {
		logger.Infof("Ignoring: %s: %s", key, err.Error())
		return nil
	}

	shellJobs(shellVars, stg.Jobs.Before)
	defer shellJobs(shellVars, stg.Jobs.After)

	if obj := stg.GetInput(); obj != nil {
		fmt.Println(string(obj))
	}

	installed, err := stg.Status()
	if !installed {
		logger.Infof("Installing: %s", key)
		if err := stg.Install(); err != nil {
			logger.Fatalf("Failed to install %s: %s", key, err)
			return err
		}
		logger.Infof("Installed: %s", key)
		return nil
	}

	// upgrade if already installed
	logger.Infof("Upgrading: %s", key)
	if err = stg.Upgrade(); err != nil {
		logger.Fatalf("Failed to upgrade %s: %s", key, err)
		return err
	}
	logger.Infof("Upgraded: %s", key)
	return nil
}
