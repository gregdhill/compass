package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"sync"

	flags "github.com/jessevdk/go-flags"
	"github.com/monax/compass/core"
	"github.com/monax/compass/helm"
	yaml "gopkg.in/yaml.v2"
)

var opts struct {
	Args struct {
		Scroll string `description:"YAML pipeline file."`
	} `positional-args:"yes" required:"yes"`
	Destroy    bool     `short:"d" long:"destroy" description:"Purge all releases, top-down"`
	Export     bool     `short:"e" long:"export" description:"Render JSON marshalled values"`
	Import     []string `short:"i" long:"import" description:"YAML file with key:value mappings"`
	TillerName string   `short:"n" long:"namespace" description:"Namespace to search for Tiller" default:"kube-system"`
	TillerPort string   `short:"p" long:"port" description:"Port to connect to Tiller" default:"44134"`
	Verbose    bool     `short:"v" long:"verbose" description:"Show verbose debug information"`
	Until      string   `short:"u" long:"until" description:"Deploy chart and dependencies"`
}

func start(args []string) error {
	_, err := flags.ParseArgs(&opts, args)
	if err != nil {
		return err
	}

	pipeline := opts.Args.Scroll
	p := core.Pipeline{}
	data, err := ioutil.ReadFile(pipeline)
	if err != nil {
		return err
	}

	err = yaml.Unmarshal([]byte(data), &p)
	if err != nil {
		return err
	}

	values := core.Values(make(map[string]string, len(p.Values)))
	values.Append(p.Values)
	for _, i := range opts.Import {
		err = values.Render(i)
		if err != nil {
			return err
		}
	}
	err = p.Lint(values)
	if err != nil {
		return err
	}

	if opts.Export {
		valOut, err := json.Marshal(values)
		if err != nil {
			return err
		}
		fmt.Println(string(valOut))
		return nil
	}

	verbose := opts.Verbose
	client := helm.Setup(opts.TillerName, opts.TillerPort)
	defer client.Close()

	stages := p.Stages
	if len(stages) == 0 {
		return fmt.Errorf("no charts specified")
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
		return nil
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

		return nil
	}

	// run full workflow
	wg.Add(len(stages))
	for key, stage := range stages {
		go func(stage *core.Stage, key string) {
			defer wg.Done()
			stage.Create(client, key, values, verbose, d)
		}(stage, key)
	}
	return nil
}

func main() {
	err := start(os.Args[1:])
	if err != nil {
		panic(err)
	}
}
