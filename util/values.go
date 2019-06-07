package util

import (
	"fmt"
	"io/ioutil"
	"text/template"

	yaml "gopkg.in/yaml.v2"
)

// Values represents string mappings for go variables
type Values map[string]string

// Append overrides the current map with a new set of values
func (v Values) Append(add map[string]string) {
	for key, value := range add {
		v[key] = value
	}
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
func (v Values) FromTemplate(fileName string, funcs template.FuncMap) error {
	data, err := Render(fileName, v, funcs)
	if err != nil {
		return err
	}

	return v.FromBytes(data)
}

// ToSlice converts key:value to key=value
func (v Values) ToSlice() []string {
	values := make([]string, 0)
	for key, value := range v {
		values = append(values, fmt.Sprintf("%s=%s", key, value))
	}
	return values
}

// Cascade returns the first non empty value
func (v Values) Cascade(current, name, field string) string {
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
