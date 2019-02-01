package main

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
)

func deleteDep(index string, deps []string) []string {
	for i, j := range deps {
		if j == index {
			deps = append(deps[:i], deps[i+1:]...)
		}
	}
	return deps
}

func generate(name string, data, out *[]byte, values map[string]string) {
	funcMap := template.FuncMap{
		"digest": dockerHash,
		"remove": removePattern,
	}

	t, err := template.New(name).Funcs(funcMap).Parse(string(*data))
	if err != nil {
		log.Fatalf("failed to render %s : %s\n", name, err)
	}

	buf := new(bytes.Buffer)
	err = t.Execute(buf, values)
	if err != nil {
		log.Fatalf("failed to render %s : %s\n", name, err)
	}
	*out = append(*out, buf.Bytes()...)
}

func shellVars(vals map[string]string) []string {
	envs := make([]string, len(vals))
	for key, value := range vals {
		envs = append(envs, fmt.Sprintf("%s=%s", key, value))
	}
	return envs
}

func shellJobs(values []string, jobs []string, verbose bool) error {
	for _, command := range jobs {
		log.Printf("running job: %s\n", command)
		args := strings.Fields(command)
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Env = append(values, os.Environ()...)
		stdout, err := cmd.Output()
		if verbose && stdout != nil {
			fmt.Println(string(stdout))
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func checkRequires(values map[string]string, reqs []string) error {
	for _, r := range reqs {
		if _, exists := values[r]; !exists {
			return errors.New("requirement not met")
		}
	}
	return nil
}

func cpVals(prev map[string]string) map[string]string {
	// copy values from main for individual chart
	values := make(map[string]string, len(prev))
	for k, v := range prev {
		values[k] = v
	}
	return values
}

func rmChart(key string, helm Helm, chart Chart, values map[string]string, finished chan string, wg *sync.WaitGroup, verbose bool, deps int) error {
	defer wg.Done()
	defer func() {
		for _, d := range chart.Depends {
			finished <- d
		}
	}()

	err := checkRequires(values, chart.Requires)
	if err != nil {
		return err
	}

	for deps > 0 {
		dep := <-finished
		if key == dep {
			deps--
		} else {
			finished <- dep
		}
	}

	log.Printf("deleting %s\n", chart.Release)
	return deleteChart(helm.client, chart.Release)
}

func mkChart(key string, helm Helm, chart Chart, mainVals map[string]string, finished chan string, wg *sync.WaitGroup, verbose bool) error {
	defer wg.Done()
	defer func() { finished <- key }()

	_, err := releaseStatus(helm.client, chart.Release)
	if err == nil && chart.Abandon {
		return errors.New("chart already installed")
	}

	values := cpVals(mainVals)
	mergeVals(values, loadVals(chart.Values, nil))
	mergeVals(values, map[string]string{"namespace": chart.Namespace})
	mergeVals(values, map[string]string{"release": chart.Release})

	err = checkRequires(values, chart.Requires)
	if err != nil {
		return err
	}

	deps := chart.Depends
	for len(deps) > 0 {
		dep := <-finished
		finished <- dep
		deps = deleteDep(dep, deps)
	}

	shellJobs(shellVars(values), chart.Jobs.Before, verbose)
	defer shellJobs(shellVars(values), chart.Jobs.After, verbose)

	var out []byte
	for _, temp := range chart.Templates {
		data, read := ioutil.ReadFile(temp)
		if read != nil {
			panic(read)
		}
		generate(temp, &data, &out, values)
	}

	if verbose {
		fmt.Println(string(out))
	}

	status, err := releaseStatus(helm.client, chart.Release)
	if status == "PENDING_INSTALL" || err != nil {
		if err == nil {
			log.Printf("deleting release: %s\n", chart.Release)
			deleteChart(helm.client, chart.Release)
		}
		log.Printf("installing release: %s\n", chart.Release)
		err := installChart(helm.client, helm.envset, chart, out)
		if err != nil {
			log.Fatalf("failed to install %s : %s\n", chart.Release, err)
		}
		log.Printf("release %s installed\n", chart.Release)
		return nil
	}

	log.Printf("upgrading release: %s\n", chart.Release)
	upgradeChart(helm.client, helm.envset, chart, out)
	if err != nil {
		log.Fatalf("failed to install %s : %s\n", chart.Release, err)
	}
	log.Printf("release upgraded: %s\n", chart.Release)
	return nil
}
