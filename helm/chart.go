package helm

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

	"github.com/monax/compass/helm/docker"
)

func deleteDep(index string, deps []string) []string {
	for i, j := range deps {
		if j == index {
			deps = append(deps[:i], deps[i+1:]...)
		}
	}
	return deps
}

// Generate renders the given values template
func Generate(name string, data, out *[]byte, values map[string]string) {
	funcMap := template.FuncMap{
		"getDigest": docker.GetImageHash,
		"getAuth":   docker.GetAuthToken,
		"readEnv":   os.Getenv,
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

// Remove deletes the chart once its dependencies have been met
func (b *Bridge) Remove(key string, chart Chart, values map[string]string, verbose bool,
	wg *sync.WaitGroup, deps map[string]*sync.WaitGroup) error {

	defer wg.Done()
	defer func() {
		for _, d := range chart.Depends {
			deps[d].Done()
		}
	}()

	err := checkRequires(values, chart.Requires)
	if err != nil {
		return err
	}

	deps[key].Wait()

	log.Printf("deleting %s\n", chart.Release)
	return deleteChart(b.client, chart.Release)
}

// Make creates the chart once its dependencies have been met
func (b *Bridge) Make(key string, chart Chart, main map[string]string, verbose bool,
	wg *sync.WaitGroup, deps map[string]*sync.WaitGroup) error {

	defer wg.Done()
	defer func() { deps[key].Done() }()

	_, err := releaseStatus(b.client, chart.Release)
	if err == nil && chart.Abandon {
		return errors.New("chart already installed")
	}

	values := cpVals(main)
	MergeVals(values, LoadVals(chart.Values, nil))
	MergeVals(values, map[string]string{"namespace": chart.Namespace})
	MergeVals(values, map[string]string{"release": chart.Release})

	err = checkRequires(values, chart.Requires)
	if err != nil {
		return err
	}

	for _, dep := range chart.Depends {
		deps[dep].Wait()
	}

	shellJobs(shellVars(values), chart.Jobs.Before, verbose)
	defer shellJobs(shellVars(values), chart.Jobs.After, verbose)

	var out []byte
	for _, temp := range chart.Templates {
		data, read := ioutil.ReadFile(temp)
		if read != nil {
			panic(read)
		}
		Generate(temp, &data, &out, values)
	}

	if verbose {
		fmt.Println(string(out))
	}

	status, err := releaseStatus(b.client, chart.Release)
	if status == "PENDING_INSTALL" || err != nil {
		if err == nil {
			log.Printf("deleting release: %s\n", chart.Release)
			deleteChart(b.client, chart.Release)
		}
		log.Printf("installing release: %s\n", chart.Release)
		err := installChart(b.client, b.envset, chart, out)
		if err != nil {
			log.Fatalf("failed to install %s : %s\n", chart.Release, err)
		}
		log.Printf("release %s installed\n", chart.Release)
		return nil
	}

	log.Printf("upgrading release: %s\n", chart.Release)
	upgradeChart(b.client, b.envset, chart, out)
	if err != nil {
		log.Fatalf("failed to install %s : %s\n", chart.Release, err)
	}
	log.Printf("release upgraded: %s\n", chart.Release)
	return nil
}
