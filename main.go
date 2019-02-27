package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"sync"

	flags "github.com/jessevdk/go-flags"
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

func setField(name, chart, target, offset string, values map[string]string, empty bool) (field string) {
	fields := [3]string{
		values[fmt.Sprintf("%s_%s", name, offset)],
		values[offset],
		target,
	}

	for _, field = range fields {
		if field != "" {
			if chart != "" {
				field = fmt.Sprintf("%s-%s", field, chart)
			}
			mergeVals(values, map[string]string{fmt.Sprintf("%s_%s", name, offset): field})
			return
		}
	}

	if !empty {
		panic(fmt.Sprintf("%s chart not given %s", name, offset))
	}

	mergeVals(values, map[string]string{fmt.Sprintf("%s_%s", name, offset): target})
	return
}

func lint(p *Pipeline, values map[string]string, root string) {
	for n, c := range p.Charts {
		c.Namespace = setField(n, "", c.Namespace, "namespace", values, false)
		c.Release = setField(n, c.Name, c.Release, "release", values, false)
		c.Version = setField(n, "", c.Version, "version", values, true)
		for i, j := range c.Jobs.Before {
			c.Jobs.Before[i] = path.Join(root, j)
		}
		for i, j := range c.Jobs.After {
			c.Jobs.After[i] = path.Join(root, j)
		}
		if c.Timeout == 0 {
			c.Timeout = 300
		}
	}
}

func preRender(tpl string, values map[string]string) map[string]string {
	if tpl == "" {
		return values
	}
	data, err := ioutil.ReadFile(tpl)
	if err != nil {
		log.Fatalf("couldn't read from %s\n", tpl)
	}
	var out []byte
	generate(tpl, &data, &out, values)
	mergeVals(values, loadVals(tpl, out))

	return values
}

func postRender(values map[string]string) {
	valOut, err := json.Marshal(values)
	if err != nil {
		panic(err)
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
		log.Printf("error unmarshalling from %s: %v\n", vals, err)
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
		log.Printf("error reading from %s: %v\n", vals, err)
		return nil
	}

	return data
}

func mergeVals(prev map[string]string, next map[string]string) {
	for key, value := range next {
		prev[key] = value
	}
}

func main() {

	var opts struct {
		Args struct {
			Scroll string `description:"YAML pipeline file."`
		} `positional-args:"yes" required:"yes"`
		Destroy bool   `short:"d" long:"destroy" description:"Purge all releases, top-down"`
		Export  bool   `short:"e" long:"export" description:"Render JSON marshalled values"`
		Import  string `short:"i" long:"import" description:"YAML file with key:value mappings"`
		Verbose bool   `short:"v" long:"verbose" description:"Show verbose debug information"`
		Until   string `short:"u" long:"until" description:"Deploy chart and dependencies"`
	}

	_, err := flags.Parse(&opts)
	if err != nil {
		os.Exit(1)
	}

	pipeline := opts.Args.Scroll
	p := Pipeline{}
	data, err := ioutil.ReadFile(pipeline)
	if err != nil {
		log.Fatal(err)
	}

	dir, err := filepath.Abs(filepath.Dir(pipeline))
	if err != nil {
		log.Fatal(err)
	}

	err = yaml.Unmarshal([]byte(data), &p)
	if err != nil {
		log.Fatal(err)
	}

	values := make(map[string]string, len(p.Values))
	mergeVals(values, p.Values)
	mergeVals(values, loadVals(opts.Import, nil))
	preRender(p.Derive, values)
	lint(&p, values, dir)

	if opts.Export {
		postRender(values)
		return
	}

	verbose := opts.Verbose
	helm := setupHelm()
	defer close(helm.tiller)

	charts := p.Charts
	if len(charts) == 0 {
		log.Fatalln("no charts specified")
	}

	wgs := make(map[string]*sync.WaitGroup, len(charts))
	var wg sync.WaitGroup
	defer wg.Wait()

	// reverse workflow: helm del --purge
	if opts.Destroy {
		wg.Add(len(charts))
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

		for key, chart := range charts {
			go rmChart(key, *helm, *chart, values, verbose, &wg, wgs)
		}
		return
	}

	for key := range charts {
		// chart dependencies wait on this
		var w sync.WaitGroup
		w.Add(1)
		wgs[key] = &w
	}

	// stop at desired chart
	if opts.Until != "" {
		if _, ok := charts[opts.Until]; !ok {
			log.Fatalf("%s does not exist", opts.Until)
		}
		go mkChart(opts.Until, *helm, *charts[opts.Until], values, verbose, &wg, wgs)
		for _, dep := range charts[opts.Until].Depends {
			go mkChart(dep, *helm, *charts[dep], values, verbose, &wg, wgs)
		}
		wg.Add(len(charts[opts.Until].Depends) + 1)
		return
	}

	// run full workflow
	wg.Add(len(charts))
	for key, chart := range charts {
		go mkChart(key, *helm, *chart, values, verbose, &wg, wgs)
	}
}
