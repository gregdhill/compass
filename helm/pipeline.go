package helm

import (
	"io/ioutil"
	"log"
	"sync"

	yaml "gopkg.in/yaml.v2"
)

// Jobs represent any bash jobs that should be run as part of a release.
type Jobs struct {
	Before []string `yaml:"before"`
	After  []string `yaml:"after"`
}

// Chart represents a single stage of the deployment pipeline.
type Chart struct {
	Name      string   `yaml:"name"`      // name of chart
	Repo      string   `yaml:"repo"`      // chart repo
	Version   string   `yaml:"version"`   // chart version
	Release   string   `yaml:"release"`   // release name
	Timeout   int64    `yaml:"timeout"`   // install / upgrade wait time
	Namespace string   `yaml:"namespace"` // namespace
	Abandon   bool     `yaml:"abandon"`   // install only
	Values    string   `yaml:"values"`    // chart specific values
	Requires  []string `yaml:"requires"`  // env requirements
	Depends   []string `yaml:"depends"`   // dependencies
	Jobs      Jobs     `yaml:"jobs"`      // bash jobs
	Templates []string `yaml:"templates"` // templates
}

// Pipeline represents the complete workflow.
type Pipeline struct {
	Derive string            `yaml:"derive"`
	Charts map[string]*Chart `yaml:"charts"`
	Values map[string]string `yaml:"values"`
}

// BuildDepends generates a dependency map
func (p Pipeline) BuildDepends(reverse bool) *Depends {
	charts := p.Charts
	wgs := make(Depends, len(charts))

	if reverse {
		deps := make(map[string]int, len(charts))
		for _, chart := range charts {
			for _, d := range chart.Depends {
				deps[d]++
			}
		}
		for key := range charts {
			var w sync.WaitGroup
			w.Add(deps[key])
			wgs[key] = &w
		}
		return &wgs
	}

	for key := range charts {
		var w sync.WaitGroup
		w.Add(1)
		wgs[key] = &w
	}
	return &wgs
}

type Depends map[string]*sync.WaitGroup

func (d Depends) Wait(charts ...string) {
	for _, key := range charts {
		d[key].Wait()
	}
}

func (d Depends) Complete(charts ...string) {
	for _, key := range charts {
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
