package policy

import "strings"

// JSONPathGet is a simplified JSONPath that supports `[*]` as the array wildcard
// (always returns the first element). Returns the value and a bool indicating
// whether the path was resolved.
func JSONPathGet(obj map[string]any, path string) (any, bool) {
	parts := strings.Split(path, ".")
	var cur any = obj
	for _, p := range parts {
		if strings.HasSuffix(p, "[*]") {
			key := strings.TrimSuffix(p, "[*]")
			m, ok := cur.(map[string]any)
			if !ok {
				return nil, false
			}
			arr, ok := m[key].([]any)
			if !ok {
				return nil, false
			}
			if len(arr) == 0 {
				return nil, false
			}
			cur = arr[0]
			continue
		}
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = m[p]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}
