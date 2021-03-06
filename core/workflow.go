package core

import (
	"fmt"
	"io/ioutil"
	"os"
	"sync"
	"text/template"

	"github.com/Masterminds/sprig"
	"github.com/monax/compass/core/schema"
	"github.com/monax/compass/docker"
	"github.com/monax/compass/helm"
	"github.com/monax/compass/kube"
	"github.com/monax/compass/util"
	log "github.com/sirupsen/logrus"
	"github.com/valyala/fastjson"
)

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
func NewDepends(stages map[string]*schema.Stage, reverse bool) *Depends {
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
func Lint(wf *schema.Workflow, in util.Values) (err error) {
	for key, stage := range wf.Stages {
		if err = stage.Lint(key, &in); err != nil {
			return err
		}
	}
	return nil
}

// Connect links all of our stages to their required resources and pre-renders their input
func Connect(wf *schema.Workflow, k8s *kube.K8s, tiller *helm.Tiller, v util.Values) error {
	for _, stg := range wf.Stages {
		switch stg.Kind {
		case "kube", "kubernetes":
			stg.Connect(k8s)
		case "helm":
			stg.Connect(tiller)
		}

		out, err := util.RenderFile(stg.Template, v, RenderWith(k8s))
		if err != nil {
			return err
		}
		stg.SetInput(out)
	}

	return nil
}

// RenderWith returns the supported templating functions
func RenderWith(k8s *kube.K8s) template.FuncMap {
	compassfn := template.FuncMap{
		"getDigest":     docker.GetImageDigest,
		"getCommit":     util.GetHead,
		"fromConfigMap": k8s.FromConfigMap,
		"fromSecret":    k8s.FromSecret,
		"readEnv":       os.Getenv,
		"parseJSON": func(item string, keys ...string) (string, error) {
			if item == "" {
				return "", fmt.Errorf("no JSON provided to parse")
			}
			result := fastjson.GetString([]byte(item), keys...)
			if result == "" {
				return "", fmt.Errorf("failed to find pattern (%v) in json", keys)
			}
			return result, nil
		},
		"readFile": func(filename string) (string, error) {
			data, err := ioutil.ReadFile(filename)
			return string(data), err
		},
		"console": func(arg interface{}) error {
			log.Println(arg)
			return nil
		},
	}

	sprigfn := sprig.TxtFuncMap()
	for name, fn := range compassfn {
		sprigfn[name] = fn
	}
	return sprigfn
}

// Backward deletes each stage in reverse order
func Backward(stages map[string]*schema.Stage, input util.Values, force bool) {
	var wg sync.WaitGroup
	defer wg.Wait()

	wg.Add(len(stages))
	deps := NewDepends(stages, true)

	for key, stage := range stages {
		go func(this *schema.Stage, key string) {
			defer deps.Complete(this.Depends...) // signal anything that depends on this
			defer wg.Done()                      // main thread can continue
			deps.Wait(key)                       // wait for dependants to delete first

			Destroy(this, log.WithField("kind", stage.Kind), key, input, force)
		}(stage, key)
	}
}

// Forward processes each stage in the pipeline
func Forward(stages map[string]*schema.Stage, input util.Values, force bool) error {
	var wg sync.WaitGroup

	wg.Add(len(stages))
	deps := NewDepends(stages, false)
	if deps.IsCyclic() {
		return fmt.Errorf("cycle in dependencies")
	}

	defer wg.Wait()
	log.Infoln("Starting workflow...")
	for key, stage := range stages {
		go func(this *schema.Stage, key string) {
			defer deps.Complete(key)   // indicate thread finished
			defer wg.Done()            // main thread can continue
			deps.Wait(this.Depends...) // wait for dependencies

			Create(this, log.WithField("kind", this.Kind), key, input, force)
		}(stage, key)
	}

	return nil
}

// Until creates single resource and dependencies
func Until(stages map[string]*schema.Stage, input util.Values, force bool, target string) {
	var wg sync.WaitGroup
	defer wg.Wait()

	deps := NewDepends(stages, false)

	if _, ok := stages[target]; !ok {
		log.Fatalf("%s does not exist", target)
	}

	wg.Add(len(stages[target].Depends) + 1)
	go func(this *schema.Stage, key string) {
		defer deps.Complete(key)
		defer wg.Done()
		deps.Wait(this.Depends...)

		Create(this, log.WithField("kind", this.Kind), key, input, force)
	}(stages[target], target)

	for _, dep := range stages[target].Depends {
		go func(this *schema.Stage, key string) {
			defer deps.Complete(key)
			defer wg.Done()
			deps.Wait(this.Depends...)

			Create(this, log.WithField("kind", this.Kind), key, input, force)
		}(stages[dep], dep)
	}
}
