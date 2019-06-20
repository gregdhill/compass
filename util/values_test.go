package util

import (
	"io/ioutil"
	"os"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	yaml "gopkg.in/yaml.v2"
)

const testValues = `
key:
  value:
    name: "this"
    isit: true
`

func TestUnmarshal(t *testing.T) {
	values := Values{}
	err := yaml.Unmarshal([]byte(testValues), &values)
	assert.NoError(t, err)
	exp := Values{"key": Values{"value": Values{"isit": true, "name": "this"}}}
	assert.Equal(t, exp, values)
}

func TestAppendVals(t *testing.T) {
	prev := Values{"test1": "test1"}
	next := Values{"test1": "test1"}
	prev.Append(next)
	assert.Equal(t, 1, len(prev))
	next = Values{"test2": "test2"}
	prev.Append(next)
	assert.Equal(t, 2, len(prev))
	assert.Equal(t, 1, len(next))

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
		name     string
		field    string
		expected string
	}{
		// explicit is best
		{Values{}, "something", "object", "index", "something"},

		// prefer fully-qualified name
		{Values{"index": "this", "object.index": "that"}, "", "object", "index", "that"},

		// use generalized otherwise
		{Values{"index": "this"}, "", "object", "index", "this"},
	}

	for _, tt := range cascading {
		actual := tt.values.Cascade(tt.current, tt.name, tt.field)
		if actual != tt.expected {
			t.Errorf("expected %s, actual %s", tt.expected, actual)
		}
	}
}
