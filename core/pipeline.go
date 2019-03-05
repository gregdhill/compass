package core

import (
	"io/ioutil"
	"log"
	"sync"

	yaml "gopkg.in/yaml.v2"
)

// Pipeline represents the complete workflow.
type Pipeline struct {
	Derive string            `yaml:"derive"`
	Stages map[string]*Stage `yaml:"stages"`
	Values map[string]string `yaml:"values"`
}

// BuildDepends generates a dependency map
func (p Pipeline) BuildDepends(reverse bool) *Depends {
	stages := p.Stages
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

type Depends map[string]*sync.WaitGroup

func (d Depends) Wait(stages ...string) {
	for _, key := range stages {
		d[key].Wait()
	}
}

func (d Depends) Complete(stages ...string) {
	for _, key := range stages {
		d[key].Done()
	}
}

func LoadVals(vals string, data []byte) map[string]string {
	if vals == "" {
		return nil
	}

	if data == nil {
		data = LoadFile(vals)
	}

	values := make(map[string]string)
	err := yaml.Unmarshal([]byte(data), &values)
	if err != nil {
		log.Printf("error unmarshalling from %s: %v\n", vals, err)
		return nil
	}

	return values
}

func LoadFile(vals string) []byte {
	if vals == "" {
		return nil
	}

	data, err := ioutil.ReadFile(vals)
	if err != nil {
		log.Printf("error reading from %s: %v\n", vals, err)
		return nil
	}

	return data
}

func MergeVals(prev map[string]string, next map[string]string) {
	for key, value := range next {
		prev[key] = value
	}
}
