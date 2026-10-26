package templates

import "strings"

// protoFieldNames follows the pinned protoc-gen-go open API naming rules for
// the lowercase ASCII field DSL. Optional fields reserve synthetic oneof names.
// Keep this projection covered by compilation against the actual pinned plugin.
func protoFieldNames(names []string, optionalStart, optionalEnd int) []string {
	used := map[string]bool{
		"Reset": true, "String": true, "ProtoMessage": true,
		"Marshal": true, "Unmarshal": true, "ExtensionRangeArray": true,
		"ExtensionMap": true, "Descriptor": true,
	}
	unique := func(name string, getter bool) string {
		for used[name] || (getter && used["Get"+name]) {
			name += "_"
		}
		used[name] = true
		used["Get"+name] = getter
		return name
	}
	result := make([]string, len(names))
	for i, name := range names {
		result[i] = unique(protoCamelCase(name), true)
		if i >= optionalStart && i < optionalEnd {
			unique("X"+protoCamelCase(name), false)
		}
	}
	return result
}

func protoCamelCase(name string) string {
	var output strings.Builder
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c == '_' && i+1 < len(name) && name[i+1] >= 'a' && name[i+1] <= 'z' {
			continue
		}
		if c >= 'a' && c <= 'z' && (i == 0 || name[i-1] == '_' || (name[i-1] >= '0' && name[i-1] <= '9')) {
			c -= 'a' - 'A'
		}
		output.WriteByte(c)
	}
	return output.String()
}
