package util

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"text/template"

	yaml "gopkg.in/yaml.v2"
)

// Values represents mappings for go variables
type Values map[interface{}]interface{}

func (v Values) MarshalJSON() ([]byte, error) {
	out := make(map[string]interface{}, len(v))
	for key, value := range v {
		out[fmt.Sprintf("%v", key)] = value
	}

	return json.Marshal(out)
}

func NewValues(from map[string]string) Values {
	values := make(Values)
	for k, v := range from {
		values[k] = v
	}
	return values
}

// Append overrides the current map with a new set of values
func (v Values) Append(values Values) {
	for key, value := range values {
		switch vv := value.(type) {
		case Values:
			if v[key] == nil {
				v[key] = make(Values)
			}
			v[key].(Values).Append(vv)
		default:
			v[key] = value
		}
	}
}

// AppendStr adds mapped strings
func (v Values) AppendStr(values map[string]string) {
	for key, value := range values {
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
	data, err := RenderFile(fileName, v, funcs)
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
func (v Values) Cascade(current, key, field string) string {
	if current != "" {
		return current
	}

	switch vv := v[key].(type) {
	case map[string]string:
		return vv[field]
	case Values:
		if r, ok := vv[field].(string); ok {
			return r
		}
	}

	switch vv := v[field].(type) {
	case string:
		return vv
	}

	return ""
}

// Combine merges two string maps
func Combine(one, two map[string]string) {
	for key, value := range two {
		one[key] = value
	}
}

func (v Values) ToEnv(prefix string) {
	for key, value := range v {
		switch vv := value.(type) {
		case Values:
			if prefix == "" {
				vv.ToEnv(fmt.Sprintf("%v", key))
			} else {
				vv.ToEnv(fmt.Sprintf("%s_%v", prefix, key))
			}
		default:
			if prefix == "" {
				fmt.Printf("%v=%v\n", key, value)
			} else {
				fmt.Printf("%v_%v=%v\n", prefix, key, value)
			}
		}
	}
}
