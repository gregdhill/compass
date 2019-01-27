package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
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

func setField(name, field, offset string, values map[string]string, empty bool) string {
	if ns := values[fmt.Sprintf("%s_%s", name, offset)]; ns != "" {
		mergeVals(values, map[string]string{fmt.Sprintf("%s_%s", name, offset): ns})
		return ns
	} else if ns := values[offset]; ns != "" {
		mergeVals(values, map[string]string{fmt.Sprintf("%s_%s", name, offset): ns})
		return ns
	} else if field == "" {
		if !empty {
			panic(fmt.Sprintf("%s chart not given %s", name, offset))
		}
	}
	mergeVals(values, map[string]string{fmt.Sprintf("%s_%s", name, offset): field})
	return field
}

func lint(p *Pipeline, values map[string]string) {
	for n, c := range p.Charts {
		c.Namespace = setField(n, c.Namespace, "namespace", values, false)
		c.Release = setField(n, c.Release, "release", values, false)
		c.Version = setField(n, c.Version, "version", values, true)
	}
}

func preRender(tpl string, values map[string]string) map[string]string {
	if tpl == "" {
		return values
	}
	data, err := ioutil.ReadFile(tpl)
	if err != nil {
		panic(err)
	}
	var out []byte
	generate(&data, &out, values)
	mergeVals(values, loadVals(tpl, out))

	return values
}

func postRender(values map[string]string) {
	valOut, err := json.Marshal(values)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(string(valOut))
}

func loadVals(vals string, data []byte) map[string]string {
	if vals == "" {
		return nil
	}

	if data == nil {
		data = loadFile(vals)
	}

	values := make(map[string]string)
	err := yaml.Unmarshal([]byte(data), &values)
	if err != nil {
		fmt.Printf("Error unmarshalling from %s: %v\n", vals, err)
		return nil
	}

	return values
}

func loadFile(vals string) []byte {
	if vals == "" {
		return nil
	}

	data, err := ioutil.ReadFile(vals)
	if err != nil {
		fmt.Printf("Error reading from %s: %v\n", vals, err)
		return nil
	}

	return data
}

func mergeVals(prev map[string]string, next map[string]string) {
	for key, value := range next {
		prev[key] = value
	}
}

func cUsage() {
	fmt.Printf("Usage: %s [OPTIONS] scroll ...\n", os.Args[0])
	flag.PrintDefaults()
}

func main() {
	var envFile string
	var out bool
	var plan bool
	flag.StringVar(&envFile, "env", "", "Environment file with key:value mappings.")
	flag.BoolVar(&out, "out", false, "Render initial json marshalled values.")
	flag.BoolVar(&plan, "plan", false, "Generate chart values without deploying.")
	flag.Parse()
	flag.Usage = cUsage

	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(1)
	}

	pipeline := flag.Arg(0)
	if pipeline == "" {
		panic("No pipeline file specified")
	}

	if envFile == "" {
		fmt.Println("Not using environment file")
	}

	p := Pipeline{}
	data, err := ioutil.ReadFile(pipeline)
	if err != nil {
		panic(err)
	}

	err = yaml.Unmarshal([]byte(data), &p)
	if err != nil {
		panic(err)
	}

	values := make(map[string]string, len(p.Values))
	mergeVals(values, p.Values)
	mergeVals(values, loadVals(envFile, nil))
	preRender(p.Derive, values)
	lint(&p, values)

	if out {
		postRender(values)
		return
	}

	helm := setupHelm()
	defer close(helm.tiller)

	charts := p.Charts
	if len(charts) == 0 {
		panic("No charts specified")
	}

	finished := make(chan string, len(charts))
	var wg sync.WaitGroup
	wg.Add(len(charts))

	for key, chart := range charts {
		go newChart(key, *helm, *chart, values, finished, &wg, plan)
	}

	wg.Wait()
}
