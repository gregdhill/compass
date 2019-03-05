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
	"github.com/monax/compass/core"
	"github.com/monax/compass/core/helm"
	yaml "gopkg.in/yaml.v2"
)

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
			core.MergeVals(values, map[string]string{fmt.Sprintf("%s_%s", name, offset): field})
			return
		}
	}

	if !empty {
		panic(fmt.Sprintf("%s chart not given %s", name, offset))
	}

	core.MergeVals(values, map[string]string{fmt.Sprintf("%s_%s", name, offset): target})
	return
}

func lint(p *core.Pipeline, values map[string]string, root string) {
	for n, c := range p.Stages {
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

func main() {
	var opts struct {
		Args struct {
			Scroll string `description:"YAML pipeline file."`
		} `positional-args:"yes" required:"yes"`
		Destroy    bool   `short:"d" long:"destroy" description:"Purge all releases, top-down"`
		Export     bool   `short:"e" long:"export" description:"Render JSON marshalled values"`
		Import     string `short:"i" long:"import" description:"YAML file with key:value mappings"`
		TillerName string `short:"n" long:"namespace" description:"Namespace to search for Tiller" default:"kube-system"`
		TillerPort string `short:"p" long:"port" description:"Port to connect to Tiller" default:"44134"`
		Verbose    bool   `short:"v" long:"verbose" description:"Show verbose debug information"`
		Until      string `short:"u" long:"until" description:"Deploy chart and dependencies"`
	}

	_, err := flags.Parse(&opts)
	if err != nil {
		os.Exit(1)
	}

	pipeline := opts.Args.Scroll
	p := core.Pipeline{}
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
	core.MergeVals(values, p.Values)
	core.MergeVals(values, core.LoadVals(opts.Import, nil))
	core.Extrapolate(p.Derive, values)
	lint(&p, values, dir)

	if opts.Export {
		valOut, err := json.Marshal(values)
		if err != nil {
			panic(err)
		}
		fmt.Println(string(valOut))
		return
	}

	verbose := opts.Verbose
	client := helm.Setup(opts.TillerName, opts.TillerPort)
	defer client.Close()

	stages := p.Stages
	if len(stages) == 0 {
		log.Fatalln("no charts specified")
	}

	var wg sync.WaitGroup
	defer wg.Wait()

	// reverse workflow: helm del --purge
	if opts.Destroy {
		wg.Add(len(stages))
		d := p.BuildDepends(true)

		for key, stage := range stages {
			go func(chart *core.Stage, key string) {
				defer wg.Done()
				chart.Destroy(client, key, values, verbose, d)
			}(stage, key)
		}
		return
	}

	d := p.BuildDepends(false)

	// stop at desired chart
	if opts.Until != "" {
		wg.Add(len(stages[opts.Until].Depends) + 1)
		if _, ok := stages[opts.Until]; !ok {
			log.Fatalf("%s does not exist", opts.Until)
		}

		go func(stage *core.Stage, key string) {
			defer wg.Done()
			stage.Create(client, key, values, verbose, d)
		}(stages[opts.Until], opts.Until)

		for _, dep := range stages[opts.Until].Depends {
			go func(stage *core.Stage, key string) {
				defer wg.Done()
				stage.Create(client, key, values, verbose, d)
			}(stages[dep], dep)
		}

		return
	}

	// run full workflow
	wg.Add(len(stages))
	for key, stage := range stages {
		go func(stage *core.Stage, key string) {
			defer wg.Done()
			stage.Create(client, key, values, verbose, d)
		}(stage, key)
	}
}
