// Package utils contains the utility functions for the dask-operator
package utils

import (
	"bytes"
	"encoding/json"
	"text/template"

	"github.com/Masterminds/sprig"
	dtypes "github.com/piersharding/dask-operator/types"

	"k8s.io/helm/pkg/chartutil"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// recursive call to convert map keys
func ConvertKeysToString(i interface{}) interface{} {
	switch x := i.(type) {
	case map[interface{}]interface{}:
		m2 := map[string]interface{}{}
		for k, v := range x {
			m2[k.(string)] = ConvertKeysToString(v)
		}
		return m2
	case []interface{}:
		for i, v := range x {
			x[i] = ConvertKeysToString(v)
		}
	}
	return i
}

// YamlToMap converts a YAML string to a JSON string
// The input string is a byte array, and output is a generic interface{} struct
func YamlToMap(s string) (interface{}, error) {
	log.Debugf("YamlToMap Input: %s", s)
	var body interface{}
	if err := yaml.Unmarshal([]byte(s), &body); err != nil {
		return nil, err
	}

	body = ConvertKeysToString(body)
	log.Debugf("YamlToMap Parsed: %+v", body)
	return body, nil
}

// FuncMap taken from Helm
func FuncMap() template.FuncMap {
	f := sprig.TxtFuncMap()
	delete(f, "env")
	delete(f, "expandenv")

	// Add some extra functionality
	extra := template.FuncMap{
		"toToml":   chartutil.ToToml,
		"toYaml":   chartutil.ToYaml,
		"fromYaml": chartutil.FromYaml,
		"toJson":   chartutil.ToJson,
		"fromJson": chartutil.FromJson,

		// This is a placeholder for the "include" function, which is
		// late-bound to a template. By declaring it here, we preserve the
		// integrity of the linter.
		"include":  func(string, interface{}) string { return "not implemented" },
		"required": func(string, interface{}) interface{} { return "not implemented" },
		"tpl":      func(string, interface{}) interface{} { return "not implemented" },
	}

	for k, v := range extra {
		f[k] = v
	}

	return f
}

// ApplyTemplate take YAML template and interpolates context vars
// then returns JSON
func ApplyTemplate(s string, context dtypes.DaskContext) (string, error) {

	t := template.Must(template.New("template").Funcs(FuncMap()).Parse(s))
	var tpl bytes.Buffer
	if err := t.Execute(&tpl, context); err != nil {
		log.Debugf("Template Error: %+v\n", err)
		return "", err
	}

	yamlData := tpl.String()
	body, err := YamlToMap(yamlData)
	if err != nil {
		log.Debugf("YamlToMap Error: %+v\n", err)
		return "", err
	}
	// the YAML de-serialiser is rubbish so we have to wash it through
	// the JSON one!!!
	result, err := json.Marshal(body)
	if err != nil {
		log.Debugf("json.Marshal Error: %+v\n", err)
		return "", err
	}

	log.Debugf("Output: %s\n", result)
	return string(result), nil
}
