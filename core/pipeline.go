package core

import (
	"bytes"
	"io/ioutil"
	"os"
	"sync"
	"text/template"

	"github.com/monax/compass/docker"
	"github.com/monax/compass/helm"
	"github.com/monax/compass/kube"
	"github.com/monax/compass/util"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

// Stages represents the complete workflow.
type Stages map[string]*Stage

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
func (stg *Stages) BuildDepends(reverse bool) *Depends {
	stages := *stg
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
func (stg *Stages) Lint(in util.Values) (err error) {
	for key, stage := range *stg {
		if err = stage.Lint(key, &in); err != nil {
			return err
		}
	}
	return nil
}

// Connect links all of our stages to their required resources and pre-renders their input
func (stg *Stages) Connect(k8s *kube.K8s, input util.Values, tillerName, tillerPort string) (func(), error) {
	tiller := helm.NewClient(k8s, tillerName, tillerPort)
	closer := func() {
		tiller.Close()
	}

	for _, stg := range *stg {
		switch stg.Kind {
		case "kube", "kubernetes":
			stg.Connect(k8s)
		case "helm":
			stg.Connect(tiller)
		}

		out, err := Render(stg.Template, input, k8s)
		if err != nil {
			return closer, err
		}
		if stg.Values != nil {
			vals, err := yaml.Marshal(stg.Values)
			if err != nil {
				return closer, err
			}
			out = append(out, vals...)
		}

		stg.SetInput(out)
	}

	return closer, nil
}

// Render reads a file and templates it according to the provided functions
func Render(name string, input util.Values, k8s *kube.K8s) ([]byte, error) {
	funcs := template.FuncMap{
		"getDigest":     docker.GetImageHash,
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
func (stg *Stages) Destroy(input util.Values, force bool) {
	var wg sync.WaitGroup
	defer wg.Wait()

	stages := *stg
	wg.Add(len(stages))
	d := stg.BuildDepends(true)

	for key, stage := range stages {
		go func(stg *Stage, key string) {
			logger := log.WithField("kind", stage.Kind)
			defer wg.Done()
			stg.Backward(logger, key, input, d, force)
		}(stage, key)
	}
}

// Run processes each stage in the pipeline
func (stg *Stages) Run(input util.Values, force bool) {
	var wg sync.WaitGroup
	defer wg.Wait()

	stages := *stg
	wg.Add(len(stages))
	d := stg.BuildDepends(false)

	log.Infoln("Starting workflow...")
	for key, stage := range stages {
		go func(stage *Stage, key string) {
			logger := log.WithField("kind", stage.Kind)
			defer wg.Done()
			stage.Forward(logger, key, input, d, force)
		}(stage, key)
	}
}

// Until creates single resource and dependencies
func (stg *Stages) Until(input util.Values, force bool, target string) {
	var wg sync.WaitGroup
	defer wg.Wait()

	stages := *stg
	d := stg.BuildDepends(false)

	if _, ok := stages[target]; !ok {
		log.Fatalf("%s does not exist", target)
	}

	wg.Add(len(stages[target].Depends) + 1)
	go func(stage *Stage, key string) {
		logger := log.WithField("kind", stage.Kind)
		defer wg.Done()
		stage.Forward(logger, key, input, d, force)
	}(stages[target], target)

	for _, dep := range stages[target].Depends {
		go func(stage *Stage, key string) {
			logger := log.WithField("kind", stage.Kind)
			defer wg.Done()
			stage.Forward(logger, key, input, d, force)
		}(stages[dep], dep)
	}
}
