package core

import (
	"fmt"
	"io/ioutil"
	"sync"

	yaml "gopkg.in/yaml.v2"
)

// Pipeline represents the complete workflow.
type Pipeline struct {
	Stages map[string]*Stage `yaml:"stages"`
	Values Values            `yaml:"values"`
}

// BuildDepends generates a dependency map
func (p Pipeline) BuildDepends(reverse bool) *Depends {
	stages := p.Stages
	wgs := make(Depends, len(stages))

	if reverse {
		deps := make(map[string]int, len(stages))
		for _, chart := range stages {
			for _, d := range chart.Depends {
				deps[d]++
			}
		}
		for key := range stages {
			var w sync.WaitGroup
			w.Add(deps[key])
			wgs[key] = &w
		}
		return &wgs
	}

	for key := range stages {
		var w sync.WaitGroup
		w.Add(1)
		wgs[key] = &w
	}
	return &wgs
}

func (p *Pipeline) Lint(in Values) error {
	for key, stage := range p.Stages {
		stage.Namespace = in.Validate(key, "namespace", stage.Namespace)
		if stage.Namespace == "" {
			return fmt.Errorf("namespace for '%s' is empty", key)
		}
		stage.Release = in.Validate(key, "release", stage.Release)
		if stage.Release == "" {
			return fmt.Errorf("release for '%s' is empty", key)
		}
		stage.Release = fmt.Sprintf("%s-%s", stage.Release, key)
		in[fmt.Sprintf("%s_release", key)] = stage.Release
		stage.Version = in.Validate(key, "version", stage.Version)

		if stage.Timeout == 0 {
			stage.Timeout = 300
		}
	}
	return nil
}

// Depends implements a mapped waitgroup for dependencies
type Depends map[string]*sync.WaitGroup

// Wait on given waitgroups
func (d Depends) Wait(stages ...string) {
	for _, key := range stages {
		d[key].Wait()
	}
}

// Complete given waitgroups
func (d Depends) Complete(stages ...string) {
	for _, key := range stages {
		d[key].Done()
	}
}

// Values represents string mappings for go variables
type Values map[string]string

// FromFile reads more key:value mappings from a file
func (v Values) FromFile(file string) error {
	if file == "" {
		return nil
	}

	data, err := LoadFile(file)
	if err != nil {
		return err
	}

	err = v.FromBytes(data)
	if err != nil {
		return err
	}

	return nil
}

// FromBytes reads more key:value mappings from a byte slice
func (v Values) FromBytes(data []byte) error {
	if data == nil {
		return nil
	}

	values := make(map[string]string)
	err := yaml.Unmarshal(data, &values)
	if err != nil {
		return err
	}

	v.Append(values)
	return nil
}

// Render templates more values from the given file
func (v Values) Render(file string) error {
	if file == "" {
		return nil
	}

	data, err := LoadFile(file)
	if err != nil {
		return err
	}

	var out []byte
	err = Generate(file, &data, &out, v)
	if err != nil {
		return err
	}
	err = v.FromBytes(out)
	if err != nil {
		return err
	}
	return nil
}

// Append overrides the current map with a new set of values
func (v Values) Append(add map[string]string) {
	for key, value := range add {
		v[key] = value
	}
}

// ToSlice converts key:value to key=value
func (v Values) ToSlice() []string {
	values := make([]string, len(v))
	for key, value := range v {
		values = append(values, fmt.Sprintf("%s=%s", key, value))
	}
	return values
}

// Duplicate copies values into a new map
func (v Values) Duplicate() Values {
	values := make(map[string]string, len(v))
	for key, value := range v {
		values[key] = value
	}
	return values
}

func (values Values) Validate(name, field, current string) string {
	cascade := [3]string{
		values[fmt.Sprintf("%s_%s", name, field)],
		values[field],
		current,
	}

	for _, opt := range cascade {
		if opt != "" {
			return opt
		}
	}

	return ""
}

// LoadFile reads a file
func LoadFile(file string) ([]byte, error) {
	if file == "" {
		return nil, nil
	}

	data, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	return data, nil
}
