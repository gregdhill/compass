package util

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"text/template"

	yaml "gopkg.in/yaml.v2"
)

// Values represents string mappings for go variables
type Values map[string]string

// FromFile reads more key:value mappings from a file
func (v Values) FromFile(fileName string) error {
	if fileName == "" {
		return nil
	}

	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		return err
	}

	err = v.FromBytes(data)
	if err != nil {
		return err
	}

	return nil
}

// FromTemplate reads more key:value mappings from a templated file
func (v Values) FromTemplate(fileName string, tpl template.FuncMap) error {
	data, err := Template(fileName, v, tpl)
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

// Cascade returns the first non empty value
func (v Values) Cascade(name, field, current string) string {
	options := [3]string{
		current,
		v[fmt.Sprintf("%s_%s", name, field)],
		v[field],
	}

	for _, opt := range options {
		if opt != "" {
			v[fmt.Sprintf("%s_%s", name, field)] = opt
			return opt
		}
	}
	return ""
}

// Template reads a file and renders it according to the provided functions
func Template(name string, input map[string]string, tpl template.FuncMap) ([]byte, error) {
	if name == "" {
		return nil, nil
	}

	data, err := ioutil.ReadFile(name)
	if err != nil {
		return nil, err
	}

	buf := new(bytes.Buffer)
	t, err := template.New(name).Funcs(tpl).Parse(string(data))
	if err != nil {
		return nil, err
	}
	err = t.Execute(buf, input)
	return buf.Bytes(), err
}
