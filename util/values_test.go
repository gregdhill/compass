package util

import (
	"io/ioutil"
	"os"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	yaml "gopkg.in/yaml.v2"
)

const testValues1 = `
key:
  value:
    name: "this"
    isit: true
    nested:
      birds:
        sleep: together
    what:
    - foo: one
      bar: two
    - foo: three
      bar: four
`

const testValues2 = `
key:
  value:
    hello: world
    name: that
    nested:
      birds:
        fly: false

input: text
`

func TestAppendVals(t *testing.T) {
	t.Run("Overwrite", func(t *testing.T) {
		prev := Values{"foo": "bar"}
		next := Values{"foo": "tar"}
		prev.Append(next)
		assert.Equal(t, 1, len(prev))
		assert.Equal(t, Values{"foo": "tar"}, prev)
	})

	t.Run("Append", func(t *testing.T) {
		prev := Values{"foo": "one"}
		next := Values{"bar": "two"}
		prev.Append(next)
		assert.Equal(t, 2, len(prev))
		assert.Equal(t, Values{"foo": "one", "bar": "two"}, prev)
		assert.Equal(t, 1, len(next))
	})
}

func TestUnmarshal(t *testing.T) {
	t.Run("Unmarshal yamls", func(t *testing.T) {
		values := new(Values)
		err := yaml.Unmarshal([]byte(testValues1), values)
		assert.NoError(t, err)
		exp := Values{"key": Values{"value": Values{"isit": true, "name": "this", "nested": Values{"birds": Values{"sleep": "together"}}, "what": []interface{}{Values{"foo": "one", "bar": "two"}, Values{"foo": "three", "bar": "four"}}}}}
		assert.Equal(t, exp, *values)
	})

	t.Run("Combine yamls", func(t *testing.T) {
		first := new(Values)
		err := yaml.Unmarshal([]byte(testValues1), first)
		assert.NoError(t, err)

		second := new(Values)
		err = yaml.Unmarshal([]byte(testValues2), second)
		assert.NoError(t, err)

		first.Append(*second)
		exp := Values{"input": "text", "key": Values{"value": Values{"isit": true, "hello": "world", "name": "that", "nested": Values{"birds": Values{"sleep": "together", "fly": false}}, "what": []interface{}{Values{"foo": "one", "bar": "two"}, Values{"foo": "three", "bar": "four"}}}}}
		assert.Equal(t, exp, *first)
	})
}

func TestFromBytes(t *testing.T) {
	vals := Values{}
	err := vals.FromBytes([]byte("key: value"))
	assert.NoError(t, err)
	assert.Equal(t, "value", vals["key"])
}

func TestFromFile(t *testing.T) {
	vals := Values{}
	f, err := ioutil.TempFile("", "values.yaml")
	assert.NoError(t, err)
	_, err = f.WriteString("key: value")
	assert.NoError(t, err)
	assert.NoError(t, f.Close())
	defer os.Remove(f.Name())

	err = vals.FromFile(f.Name())
	assert.NoError(t, err)
	assert.Equal(t, "value", vals["key"])
}

func TestFromTemplate(t *testing.T) {
	vals := Values{"key1": "value1"}
	f, err := ioutil.TempFile("", "values.yaml")
	assert.NoError(t, err)
	_, err = f.WriteString("key2: {{ .key1 }}")
	assert.NoError(t, err)
	assert.NoError(t, f.Close())
	defer os.Remove(f.Name())

	err = vals.FromTemplate(f.Name(), template.FuncMap{})
	assert.NoError(t, err)
	assert.Equal(t, "value1", vals["key2"])
}

func TestSliceVars(t *testing.T) {
	vals := Values{"dep1": "dep2"}
	actual := vals.ToSlice()
	assert.Equal(t, len(vals), len(actual))
}

func TestCascade(t *testing.T) {
	var cascading = []struct {
		values   Values
		current  string
		key      string
		field    string
		expected string
	}{
		// explicit is best
		{Values{}, "new", "api", "version", "new"},

		// prefer fully-qualified name
		{map[interface{}]interface{}{"api": map[interface{}]string{"version": "old"}}, "", "api", "version", "old"},
	}

	for _, tt := range cascading {
		actual := tt.values.Cascade(tt.current, tt.key, tt.field)
		assert.NotEmpty(t, actual)
		assert.Equal(t, tt.expected, actual)
	}
}
