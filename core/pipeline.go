package core

import (
	"bytes"
	"io/ioutil"
	"log"
	"os"
	"sync"
	"text/template"

	"github.com/monax/compass/docker"
	"github.com/monax/compass/helm"
	"github.com/monax/compass/kube"
	"github.com/monax/compass/util"
)

// Pipeline represents the complete workflow.
type Pipeline struct {
	Values util.Values       `yaml:"values"`
	Stages map[string]*Stage `yaml:"stages"`
}

// Depends implements a mapped waitgroup for dependencies
type Depends map[string]*sync.WaitGroup

// Wait on given waitgroups
func (d Depends) Wait(stages ...string) {
	for _, key := range stages {
		d[key].Wait()
	}
}

// Complete given waitgroups
func (d Depends) Complete(stages ...string) {
	for _, key := range stages {
		d[key].Done()
	}
}

// BuildDepends generates a dependency map
func (pl *Pipeline) BuildDepends(reverse bool) *Depends {
	stages := pl.Stages
	wgs := make(Depends, len(stages))

	if reverse {
		deps := make(map[string]int, len(stages))
		for _, chart := range stages {
			for _, d := range chart.Depends {
				deps[d]++
			}
		}
		for key := range stages {
			var w sync.WaitGroup
			w.Add(deps[key])
			wgs[key] = &w
		}
		return &wgs
	}

	for key := range stages {
		var w sync.WaitGroup
		w.Add(1)
		wgs[key] = &w
	}
	return &wgs
}

// Lint all the stages in our pipeline
func (pl *Pipeline) Lint(in util.Values) (err error) {
	for key, stage := range pl.Stages {
		if err = stage.Lint(key, &in); err != nil {
			return err
		}
	}
	return nil
}

// Connect links all of our stages to their required resources
func (pl *Pipeline) Connect(tillerName, tillerPort string) (*kube.K8s, func()) {
	k8s := kube.NewK8s()
	bridge := helm.Setup(k8s, tillerName, tillerPort)

	for _, stg := range pl.Stages {
		stg.K8s = k8s
		switch stg.Kind {
		case "kube", "kubernetes":
			stg.Connect(k8s)
		case "helm":
			stg.Connect(bridge)
		}
	}

	return k8s, bridge.Close
}

// Template reads a file and renders it according to the provided functions
func Template(name string, input map[string]string, k8s *kube.K8s) ([]byte, error) {
	funcs := template.FuncMap{
		"getDigest":     docker.GetImageHash,
		"getAuth":       docker.GetAuthToken,
		"fromConfigMap": k8s.FromConfigMap,
		"fromSecret":    k8s.FromSecret,
		"parseJSON":     kube.ParseJSON,
		"readEnv":       os.Getenv,
		"readFile": func(filename string) (string, error) {
			data, err := ioutil.ReadFile(filename)
			return string(data), err
		},
	}

	if name == "" {
		return nil, nil
	}

	data, err := ioutil.ReadFile(name)
	if err != nil {
		return nil, err
	}

	buf := new(bytes.Buffer)
	t, err := template.New(name).Funcs(funcs).Parse(string(data))
	if err != nil {
		return nil, err
	}
	err = t.Execute(buf, input)
	return buf.Bytes(), err
}

// Destroy deletes each stage in reverse order
func (pl *Pipeline) Destroy(input util.Values, force, verbose bool) {
	var wg sync.WaitGroup
	defer wg.Wait()

	stages := pl.Stages
	wg.Add(len(stages))
	d := pl.BuildDepends(true)

	for key, stage := range stages {
		go func(stg *Stage, key string) {
			defer wg.Done()
			stg.Backward(key, input, d, force, verbose)
		}(stage, key)
	}
}

// Run processes each stage in the pipeline
func (pl *Pipeline) Run(input util.Values, force, verbose bool) {
	var wg sync.WaitGroup
	defer wg.Wait()

	stages := pl.Stages
	wg.Add(len(stages))
	d := pl.BuildDepends(false)

	for key, stage := range stages {
		go func(stage *Stage, key string) {
			defer wg.Done()
			stage.Forward(key, input, d, force, verbose)
		}(stage, key)
	}
}

// Until creates single resource and dependencies
func (pl *Pipeline) Until(input util.Values, force, verbose bool, target string) {
	var wg sync.WaitGroup
	defer wg.Wait()

	stages := pl.Stages
	d := pl.BuildDepends(false)

	if _, ok := stages[target]; !ok {
		log.Fatalf("%s does not exist", target)
	}

	wg.Add(len(stages[target].Depends) + 1)
	go func(stage *Stage, key string) {
		defer wg.Done()
		stage.Forward(key, input, d, force, verbose)
	}(stages[target], target)

	for _, dep := range stages[target].Depends {
		go func(stage *Stage, key string) {
			defer wg.Done()
			stage.Forward(key, input, d, force, verbose)
		}(stages[dep], dep)
	}
}
