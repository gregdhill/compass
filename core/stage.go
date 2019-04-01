package core

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/monax/compass/helm"
	"github.com/monax/compass/kube"
	"github.com/monax/compass/util"
)

// Jobs represent any shell scripts
type Jobs struct {
	Before []string `yaml:"before"`
	After  []string `yaml:"after"`
}

// Stage represents a single part of the deployment pipeline
type Stage struct {
	Abandon  bool        `yaml:"abandon"`  // install only
	Depends  []string    `yaml:"depends"`  // dependencies
	Input    string      `yaml:"input"`    // template file
	Jobs     Jobs        `yaml:"jobs"`     // bash jobs
	Kind     string      `yaml:"kind"`     // type of deploy
	Requires []string    `yaml:"requires"` // env requirements
	Remove   bool        `yaml:"remove"`   // delete instead
	Values   util.Values `yaml:"values"`   // additional values
	Render   func(string, util.Values) ([]byte, error)
	Resource
}

// UnmarshalYAML allows us to determine the type of our resource
func (stg *Stage) UnmarshalYAML(unmarshal func(interface{}) error) error {
	this := make(map[string]interface{}, 0)
	if err := unmarshal(&this); err != nil {
		return err
	}
	mapstructure.Decode(this, stg)

	switch this["kind"] {
	case "kube", "kubernetes":
		var km kube.Manifest
		km.Timeout = 300
		mapstructure.Decode(this, &km)
		stg.Resource = &km
	case "helm":
		var hc helm.Chart
		hc.Timeout = 300
		mapstructure.Decode(this, &hc)
		stg.Resource = &hc
	default:
		return fmt.Errorf("kind '%s' unknown", this["kind"])
	}

	stg.Render = func(string, util.Values) ([]byte, error) {
		return nil, nil
	}

	return nil
}

// Resource is the thing to be created / destroyed
type Resource interface {
	Lint(string, *util.Values) error
	Status() bool
	Install() error
	Upgrade() error
	Delete() error
	Connect(interface{})
	SetInput([]byte)
}

func shellJobs(values []string, jobs []string, verbose bool) {
	for _, command := range jobs {
		log.Printf("running job: %s\n", command)
		args := strings.Fields(command)
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Env = append(values, os.Environ()...)
		stdout, err := cmd.Output()
		if verbose && stdout != nil {
			fmt.Println(string(stdout))
		}
		if err != nil {
			panic(fmt.Errorf("job '%s' exited with error: %v", command, err))
		}
	}
}

func checkRequires(values map[string]string, reqs []string) string {
	for _, r := range reqs {
		if _, exists := values[r]; !exists {
			return r
		}
	}
	return ""
}

// Backward pass over the graph
func (stg *Stage) Backward(key string, global util.Values, deps *Depends, force, verbose bool) error {
	defer deps.Complete(stg.Depends...) // signal its dependencies once finished

	// only continue if required variables are set
	if req := checkRequires(global, stg.Requires); req != "" {
		log.Printf("[%s] ignoring: %s, requirement '%s' not met\n", stg.Kind, key, req)
		return nil
	}

	// don't delete by default
	if !force && stg.Abandon {
		log.Printf("[%s] ignoring: %s\n", stg.Kind, key)
		return fmt.Errorf("[%s] not deleting stage %s", stg.Kind, key)
	}

	// wait for dependants to delete first
	deps.Wait(key)
	log.Printf("[%s] deleting: %s\n", stg.Kind, key)

	out, err := stg.Render(stg.Input, global)
	if err != nil {
		panic(err)
	} else if out != nil {
		stg.SetInput(out)
	}

	return stg.Delete()
}

// Forward pass over the graph
func (stg *Stage) Forward(key string, global util.Values, deps *Depends, force, verbose bool) error {
	defer deps.Complete(key) // signal this finished

	// stop if already installed and abandoned
	installed, _ := stg.Status()
	if installed && !force && stg.Abandon {
		log.Printf("[%s] ignoring: %s\n", stg.Kind, key)
		return nil
	}

	local := global.Duplicate()
	local.Append(stg.Values)
	shellVars := local.ToSlice()
	if req := checkRequires(local, stg.Requires); req != "" {
		log.Printf("[%s] ignoring: %s, requirement '%s' not met\n", stg.Kind, key, req)
		return nil
	}

	// wait for dependencies
	deps.Wait(stg.Depends...)

	shellJobs(shellVars, stg.Jobs.Before, verbose)
	defer shellJobs(shellVars, stg.Jobs.After, verbose)

	out, err := stg.Render(stg.Input, global)
	if err != nil {
		panic(err)
	} else if out != nil {
		stg.SetInput(out)
		if verbose {
			fmt.Println(string(out))
		}
	}

	if stg.Remove {
		return stg.Delete()
	}

	installed, err = stg.Status()
	if !installed {
		log.Printf("[%s] installing: %s\n", stg.Kind, key)
		if err := stg.Install(); err != nil {
			log.Fatalf("[%s] failed to install %s: %s\n", stg.Kind, key, err)
			return err
		}
		log.Printf("[%s] installed: %s\n", stg.Kind, key)
		return nil
	}

	// upgrade if already installed
	log.Printf("[%s] upgrading: %s\n", stg.Kind, key)
	if err = stg.Upgrade(); err != nil {
		log.Fatalf("[%s] failed to upgrade %s: %s\n", stg.Kind, key, err)
		return err
	}
	log.Printf("[%s] upgraded: %s\n", stg.Kind, key)
	return nil
}
