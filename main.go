package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"syscall"

	flags "github.com/jessevdk/go-flags"
	"github.com/monax/compass/core"
	"github.com/monax/compass/util"
	yaml "gopkg.in/yaml.v2"
)

var opts struct {
	Args struct {
		Scroll string `description:"YAML formatted pipeline file"`
	} `positional-args:"yes" required:"yes"`
	Destroy    bool              `short:"d" long:"destroy" description:"Purge all releases, top-down"`
	Export     bool              `short:"e" long:"export" description:"Render JSON marshalled values"`
	Force      bool              `short:"f" long:"force" description:"Force install / upgrade / delete"`
	Import     []string          `short:"i" long:"import" description:"YAML file with key:value mappings"`
	TillerName string            `short:"n" long:"namespace" description:"Namespace to search for Tiller" default:"kube-system"`
	TillerPort string            `short:"p" long:"port" description:"Port to connect to Tiller" default:"44134"`
	Value      map[string]string `long:"value" description:"Extra values to append to the pipeline"`
	Verbose    bool              `short:"v" long:"verbose" description:"Show verbose debug information"`
	Until      string            `short:"u" long:"until" description:"Deploy chart and dependencies"`
}

func start(args []string) error {
	_, err := flags.ParseArgs(&opts, args)
	if err != nil {
		return err
	}

	// open scroll
	scroll := opts.Args.Scroll
	data, err := ioutil.ReadFile(scroll)
	if err != nil {
		return err
	}

	// build workflow
	pipe := core.Pipeline{}
	err = yaml.Unmarshal([]byte(data), &pipe)
	if err != nil {
		return err
	}

	tpl, closer := pipe.Connect(opts.TillerName, opts.TillerPort)
	defer closer()

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		closer()
		os.Exit(1)
	}()

	values := util.Values(make(map[string]string, len(pipe.Values)))
	values.Append(pipe.Values) // main values
	values.Append(opts.Value)  // explicit cli inputs

	// additional template files
	for _, i := range opts.Import {
		if err = values.FromTemplate(i, tpl); err != nil {
			return fmt.Errorf("couldn't attach import %s: %v", i, err)
		}
	}

	if err = pipe.Lint(values); err != nil {
		return err
	}

	if opts.Export {
		valOut, err := json.Marshal(values)
		if err != nil {
			return fmt.Errorf("couldn't marshal values: %v", err)
		}
		fmt.Println(string(valOut))
		return nil
	}

	force := opts.Force
	verbose := opts.Verbose
	stages := pipe.Stages
	if len(stages) == 0 {
		return fmt.Errorf("no charts specified")
	}

	// reverse workflow
	if opts.Destroy {
		pipe.Destroy(values, force, verbose)
		return nil
	}

	// stop at desired key
	if opts.Until != "" {
		pipe.Until(values, force, verbose, opts.Until)
		return nil
	}

	// run full workflow
	pipe.Run(values, force, verbose)
	return nil
}

func main() {
	err := start(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}
}
