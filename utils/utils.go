// Package utils contains the utility functions for the dask-operator
package utils

import (
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

func convertKeysToString(i interface{}) interface{} {
	switch x := i.(type) {
	case map[interface{}]interface{}:
		m2 := map[string]interface{}{}
		for k, v := range x {
			m2[k.(string)] = convertKeysToString(v)
		}
		return m2
	case []interface{}:
		for i, v := range x {
			x[i] = convertKeysToString(v)
		}
	}
	return i
}

// YamlToJSON converts a YAML string to a JSON string
// The input string is a byte array, and output is a generic interface{} struct
func YamlToJSON(s string) interface{} {
	// fmt.Printf("Input: %s\n", s)
	log.Debugf("YamlToJSON Input: %s", s)
	var body interface{}
	if err := yaml.Unmarshal([]byte(s), &body); err != nil {
		panic(err)
	}

	body = convertKeysToString(body)
	log.Debugf("YamlToJSON Parsed: %+v", body)
	// fmt.Printf("Parsed: %+v\n", body)

	// result, err := json.Marshal(body)
	// if err != nil {
	// 	panic(err)
	// 	// } else {
	// 	// 	fmt.Printf("Output: %s\n", b)
	// }
	// fmt.Printf("Parsed: %s\n", result)
	return body
}
