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
	Values map[string]string `yaml:"values"`
	Charts map[string]*Chart `yaml:"charts"`
}

func lint(p *Pipeline, values map[string]string) {
	for n, c := range p.Charts {
		if c.Namespace == "" {
			if ns := values["namespace"]; ns != "" {
				c.Namespace = ns
			} else {
				panic(fmt.Sprintf("%s chart not given namespace", c.Name))
			}
			mergeVals(values, map[string]string{fmt.Sprintf("%s_namespace", n): c.Namespace})
		}

		if c.Release == "" {
			if rs := values["release"]; rs != "" {
				c.Release = fmt.Sprintf("%s-%s", rs, c.Name)
			} else {
				panic(fmt.Sprintf("%s chart not given release", c.Name))
			}
			mergeVals(values, map[string]string{fmt.Sprintf("%s_release", n): c.Release})
		}
	}
}

func loadVals(vals string) map[string]string {
	if vals == "" {
		return nil
	}

	values := make(map[string]string)
	data, err := ioutil.ReadFile(vals)
	if err != nil {
		fmt.Printf("Error reading from %s: %v\n", vals, err)
		return nil
	}

	err = yaml.Unmarshal([]byte(data), &values)
	if err != nil {
		fmt.Printf("Error unmarshalling from %s: %v\n", vals, err)
		return nil
	}

	return values
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
	fmt.Println("Starting...")

	var envFile string
	var outFile string
	flag.StringVar(&envFile, "env", "", "Environment file with key:value mappings.")
	flag.StringVar(&outFile, "out", "", "Output primary variables.")
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
	mergeVals(values, loadVals(envFile))
	lint(&p, values)
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
		go newChart(key, *helm, *chart, values, finished, &wg)
	}

	wg.Wait()

	if outFile != "" {
		fmt.Printf("Writing values to %s\n", outFile)
		valOut, err := json.Marshal(values)
		if err != nil {
			fmt.Println(err)
			return
		}
		ioutil.WriteFile(outFile, valOut, 0644)
	}

	fmt.Println("Done")
}
