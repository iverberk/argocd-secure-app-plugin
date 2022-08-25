package main

import (
	"fmt"
	"time"
)

// mergeMaps merges two maps. In case of equal keys, both in level and name, the value
// from the second map takes precedence.
//
// CREDITS: Taken from Helm source code.
func mergeMaps(a, b map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(a))
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		if v, ok := v.(map[string]interface{}); ok {
			if bv, ok := out[k]; ok {
				if bv, ok := bv.(map[string]interface{}); ok {
					out[k] = mergeMaps(bv, v)
					continue
				}
			}
		}
		out[k] = v
	}
	return out
}

// Workaround for https://github.com/go-yaml/yaml/issues/139
//
// CREDITS: Taken from gojq source code.
func normalizeYAML(v interface{}) interface{} {
	switch v := v.(type) {
	case map[interface{}]interface{}:
		w := make(map[string]interface{}, len(v))
		for k, v := range v {
			w[fmt.Sprint(k)] = normalizeYAML(v)
		}
		return w

	case map[string]interface{}:
		w := make(map[string]interface{}, len(v))
		for k, v := range v {
			w[k] = normalizeYAML(v)
		}
		return w

	case []interface{}:
		for i, w := range v {
			v[i] = normalizeYAML(w)
		}
		return v

	// go-yaml unmarshals timestamp string to time.Time but gojq cannot handle it.
	// It is impossible to keep the original timestamp strings.
	case time.Time:
		return v.Format(time.RFC3339)

	default:
		return v
	}
}
