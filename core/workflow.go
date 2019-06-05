package core

import (
	"bytes"
	"fmt"
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

type Node struct {
	Lock  *sync.WaitGroup
	Edges []string
}

// Depends implements a mapped waitgroup for dependencies
type Depends map[string]*Node

// Wait on given waitgroups
func (d Depends) Wait(stages ...string) {
	for _, key := range stages {
		d[key].Lock.Wait()
	}
}

// Complete given waitgroups
func (d Depends) Complete(stages ...string) {
	for _, key := range stages {
		d[key].Lock.Done()
	}
}

func (d Depends) dfs(node string, visited, recStack map[string]bool) bool {
	if !visited[node] {
		visited[node] = true
		recStack[node] = true

		for _, edge := range d[node].Edges {
			if !visited[edge] && d.dfs(edge, visited, recStack) {
				return true
			} else if recStack[edge] {
				return true
			}
		}

	}
	recStack[node] = false
	return false
}

// IsCyclic returns true if there is a cycle in the graph
func (d Depends) IsCyclic() bool {
	visited := make(map[string]bool, len(d))
	recStack := make(map[string]bool, len(d))
	for key := range d {
		if d.dfs(key, visited, recStack) {
			return true
		}
	}
	return false
}

// NewDepends generates a dependency map
func (stg *Stages) NewDepends(reverse bool) *Depends {
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
			wgs[key] = &Node{
				Lock: &w,
			}
		}
		return &wgs
	}

	for key, stg := range stages {
		var w sync.WaitGroup
		w.Add(1)
		wgs[key] = &Node{
			Lock:  &w,
			Edges: stg.Depends,
		}
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
		"getDigest":     docker.GetImageDigest,
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
	deps := stg.NewDepends(true)

	for key, stage := range stages {
		go func(this *Stage, key string) {
			defer deps.Complete(this.Depends...) // signal anything that depends on this
			defer wg.Done()                      // main thread can continue
			deps.Wait(key)                       // wait for dependants to delete first

			this.Destroy(log.WithField("kind", stage.Kind), key, input, force)
		}(stage, key)
	}
}

// Run processes each stage in the pipeline
func (stg *Stages) Run(input util.Values, force bool) error {
	var wg sync.WaitGroup

	stages := *stg
	wg.Add(len(stages))
	deps := stg.NewDepends(false)
	if deps.IsCyclic() {
		return fmt.Errorf("cycle in dependencies")
	}

	defer wg.Wait()
	log.Infoln("Starting workflow...")
	for key, stage := range stages {
		go func(this *Stage, key string) {
			defer deps.Complete(key)   // indicate thread finished
			defer wg.Done()            // main thread can continue
			deps.Wait(this.Depends...) // wait for dependencies

			this.Create(log.WithField("kind", this.Kind), key, input, force)
		}(stage, key)
	}

	return nil
}

// Until creates single resource and dependencies
func (stg *Stages) Until(input util.Values, force bool, target string) {
	var wg sync.WaitGroup
	defer wg.Wait()

	stages := *stg
	deps := stg.NewDepends(false)

	if _, ok := stages[target]; !ok {
		log.Fatalf("%s does not exist", target)
	}

	wg.Add(len(stages[target].Depends) + 1)
	go func(this *Stage, key string) {
		defer deps.Complete(key)
		defer wg.Done()
		deps.Wait(this.Depends...)

		this.Create(log.WithField("kind", this.Kind), key, input, force)
	}(stages[target], target)

	for _, dep := range stages[target].Depends {
		go func(this *Stage, key string) {
			defer deps.Complete(key)
			defer wg.Done()
			deps.Wait(this.Depends...)

			this.Create(log.WithField("kind", this.Kind), key, input, force)
		}(stages[dep], dep)
	}
}
