package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"sync"

	"gopkg.in/yaml.v2"
)

// Chart represents a single stage of the deployment pipeline.
type Chart struct {
	Release   string   `yaml:"release"`
	Namespace string   `yaml:"namespace"`
	Repo      string   `yaml:"repo"`
	Name      string   `yaml:"name"`
	Abandon   bool     `yaml:"abandon"`
	Template  string   `yaml:"template"`
	Values    string   `yaml:"values"`
	Jobs      []string `yaml:"jobs"`
	Depends   []string `yaml:"depends"`
}

// Pipeline represents the complete workflow.
type Pipeline struct {
	Values map[string]string `yaml:"values"`
	Charts []Chart           `yaml:"charts"`
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

func mergeVals(prev map[interface{}]interface{}, next map[string]string) {
	for key, value := range next {
		prev[key] = value
	}
}

func bashVars(vals map[interface{}]interface{}) []string {
	envs := make([]string, len(vals))
	for key, value := range vals {
		envs = append(envs, fmt.Sprintf("%s=%s", key, value))
	}
	return envs
}

func main() {
	fmt.Println("Starting...")
	var envFile string
	flag.StringVar(&envFile, "env", "", "Environment file with key:value mappings.")
	flag.Parse()
	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(1)
	}

	pipeline := flag.Arg(0)
	if pipeline == "" {
		panic("No pipeline file specified.")
	}

	if envFile == "" {
		fmt.Println("Not using environment file.")
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

	values := make(map[interface{}]interface{})
	mergeVals(values, p.Values)
	mergeVals(values, loadVals(envFile))

	helm := setupHelm()
	defer close(helm.tiller)

	charts := p.Charts
	if len(charts) == 0 {
		panic("No charts specified.")
	}

	finished := make(chan string, len(charts))
	var wg sync.WaitGroup
	wg.Add(len(charts))

	for _, chart := range charts {
		go newChart(*helm, chart, values, finished, &wg)
	}

	wg.Wait()
	fmt.Println("Done")
}
