package util

import (
	"fmt"
	"io/ioutil"
	"text/template"

	yaml "gopkg.in/yaml.v2"
)

// Values represents mappings for go variables
type Values map[interface{}]interface{}

func NewValues(from map[string]string) Values {
	values := make(Values)
	for k, v := range from {
		values[k] = v
	}
	return values
}

// Append overrides the current map with a new set of values
func (v Values) Append(add Values) {
	for key, value := range add {
		v[key] = value
	}
}

// AppendStr adds mapped strings
func (v Values) AppendStr(add map[string]string) {
	for key, value := range add {
		v[key] = value
	}
}

// FromBytes reads more key:value mappings from a byte slice
func (v Values) FromBytes(data []byte) error {
	if data == nil {
		return nil
	}

	values := make(Values)
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
	options := [3]interface{}{
		current,
		v[fmt.Sprintf("%s.%s", name, field)],
		v[field],
	}

	for _, opt := range options {
		if opt != "" && opt != nil {
			v[fmt.Sprintf("%s_%s", name, field)] = opt
			return opt.(string)
		}
	}
	return ""
}

// Combine merges two string maps
func Combine(one, two map[string]string) {
	for key, value := range two {
		one[key] = value
	}
}
