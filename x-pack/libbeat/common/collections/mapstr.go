package collections

import (
	"strings"

	"github.com/elastic/elastic-agent-libs/mapstr"
)

// MapStrAppendString will attempt to append value to the key in map m. If the
// value at key is already a string, it will be converted into a string slice,
// and the new value will be appended. If the value at key is already a string
// slice, then the new value will be appended. Otherwise, the existing value
// (if it exists) will be overwritten with a new string slice containing the
// new value.
func MapStrAppendString(m mapstr.M, key string, value string) {
	field, err := m.GetValue(key)
	if err == mapstr.ErrKeyNotFound {
		_, _ = m.Put(key, []string{value})
		return
	}

	switch v := field.(type) {
	case string:
		_, _ = m.Put(key, []string{v, value})
	case []string:
		_, _ = m.Put(key, append(v, value))
	default:
		_, _ = m.Put(key, []string{value})
	}
}

// MapStrRemap will create a new map from m, using mappings as a guide to
// filling the new map. Only top-level key/value pairs can be remapped, it
// is not capable of remapping nested key/value pairs. The mappings are supplied
// in a map, with keys representing the keys in the existing map, and values
// representing the key in the new map. Keys specified here with dot separators
// will be set recursively. Actions may be appended with a comma to the value
// which can define behaviors that will occur when the value is being set. If
// key in the existing map is not in mappings, it will be copied into the new
// map as-is.
//
// The list of supported actions is:
// - "<old_key>": "<new_key>" - The value of new_key is set to the value of old_key
// - "<old_key>": "<new_key>,append" - The incoming value of old_key is appended as a string slice.
//
// Example:
//
// m: {"user_name": "foo", "user_alias": "bar", "github_name": "foo_github", "custom_field": "example"}
// mappings: {"user_name": "user.name", "foo": "user.alias,append", "github_name": "user.alias,append"}
// result:
//
//	{
//	  "user": {
//	    "name": "foo",
//	    "alias": ["foo", "foo_github"]
//	  },
//	  "custom_field": "example"
//	}
func MapStrRemap(m mapstr.M, mappings map[string]string) mapstr.M {
	result := mapstr.M{}

	for k, v := range m {
		if v == nil {
			continue
		}

		if target, ok := mappings[k]; ok {
			key, flag, _ := strings.Cut(target, ",")

			switch flag {
			case "append":
				switch x := v.(type) {
				case string:
					vStr, vStrOK := v.(string)
					if !vStrOK {
						continue
					}
					MapStrAppendString(result, key, vStr)
				case []any:
					for _, value := range x {
						vStr, vStrOK := value.(string)
						if !vStrOK {
							continue
						}
						MapStrAppendString(result, key, vStr)
					}
				}

			default:
				_, _ = result.Put(key, v)
			}

		} else {
			_, _ = result.Put("azure."+k, v)
		}
	}

	return result
}
