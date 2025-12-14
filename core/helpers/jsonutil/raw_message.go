package jsonutil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
)

var jsonNullBytes = []byte("null")

func IsNullRawMessage(raw json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(raw), jsonNullBytes)
}

func ReorderJSONObjectByKeys(rawObject json.RawMessage, keyOrder []string) (json.RawMessage, error) {
	trimmed := bytes.TrimSpace(rawObject)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return nil, fmt.Errorf("expected JSON object")
	}

	var object map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &object); err != nil {
		return nil, err
	}

	used := make(map[string]bool, len(object))
	var buf bytes.Buffer
	buf.WriteByte('{')

	first := true
	for _, key := range keyOrder {
		value, ok := object[key]
		if !ok {
			continue
		}
		if !first {
			buf.WriteByte(',')
		}
		first = false

		encodedKey, err := json.Marshal(key)
		if err != nil {
			return nil, err
		}
		buf.Write(encodedKey)
		buf.WriteByte(':')
		buf.Write(bytes.TrimSpace(value))

		used[key] = true
	}

	if len(used) != len(object) {
		extraKeys := make([]string, 0, len(object)-len(used))
		for key := range object {
			if !used[key] {
				extraKeys = append(extraKeys, key)
			}
		}
		sort.Strings(extraKeys)

		for _, key := range extraKeys {
			value := object[key]
			if !first {
				buf.WriteByte(',')
			}
			first = false

			encodedKey, err := json.Marshal(key)
			if err != nil {
				return nil, err
			}
			buf.Write(encodedKey)
			buf.WriteByte(':')
			buf.Write(bytes.TrimSpace(value))
		}
	}

	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// NormalizeJSONSchemaPropertiesOrder reorders the keys inside the top-level "properties"
// object to match the order of the top-level "required" array (when present).
func NormalizeJSONSchemaPropertiesOrder(schema json.RawMessage) (json.RawMessage, error) {
	trimmed := bytes.TrimSpace(schema)
	if len(trimmed) == 0 || IsNullRawMessage(trimmed) {
		return nil, nil
	}

	topKeys, err := orderedObjectKeys(trimmed)
	if err != nil {
		return schema, err
	}

	var top map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &top); err != nil {
		return schema, err
	}

	propertiesRaw, hasProperties := top["properties"]
	if !hasProperties || len(bytes.TrimSpace(propertiesRaw)) == 0 || IsNullRawMessage(propertiesRaw) {
		return schema, nil
	}

	requiredRaw, hasRequired := top["required"]
	if !hasRequired || len(bytes.TrimSpace(requiredRaw)) == 0 || IsNullRawMessage(requiredRaw) {
		return schema, nil
	}

	var required []string
	if err := json.Unmarshal(requiredRaw, &required); err != nil {
		return schema, err
	}
	if len(required) == 0 {
		return schema, nil
	}

	propertiesKeys, err := orderedObjectKeys(propertiesRaw)
	if err != nil {
		return schema, err
	}

	desiredOrder := mergeKeyOrder(required, propertiesKeys)
	if equalStringSlices(propertiesKeys, desiredOrder) {
		return schema, nil
	}

	reorderedProperties, err := ReorderJSONObjectByKeys(propertiesRaw, desiredOrder)
	if err != nil {
		return schema, err
	}

	outKeys := ensureAllKeys(topKeys, top)

	var buf bytes.Buffer
	buf.WriteByte('{')

	first := true
	for _, key := range outKeys {
		value, ok := top[key]
		if !ok {
			continue
		}
		if key == "properties" {
			value = reorderedProperties
		}

		if !first {
			buf.WriteByte(',')
		}
		first = false

		encodedKey, err := json.Marshal(key)
		if err != nil {
			return schema, err
		}
		buf.Write(encodedKey)
		buf.WriteByte(':')
		buf.Write(bytes.TrimSpace(value))
	}

	buf.WriteByte('}')
	return buf.Bytes(), nil
}

func ensureAllKeys(ordered []string, object map[string]json.RawMessage) []string {
	seen := make(map[string]bool, len(ordered))
	out := make([]string, 0, len(object))

	for _, key := range ordered {
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, key)
	}

	extras := make([]string, 0, len(object)-len(seen))
	for key := range object {
		if !seen[key] {
			extras = append(extras, key)
		}
	}
	sort.Strings(extras)
	out = append(out, extras...)

	return out
}

func mergeKeyOrder(primary []string, fallback []string) []string {
	seen := make(map[string]bool, len(primary)+len(fallback))
	out := make([]string, 0, len(primary)+len(fallback))

	for _, key := range primary {
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, key)
	}

	for _, key := range fallback {
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, key)
	}

	return out
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func orderedObjectKeys(raw json.RawMessage) ([]string, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))

	token, err := dec.Token()
	if err != nil {
		return nil, err
	}
	delim, ok := token.(json.Delim)
	if !ok || delim != '{' {
		return nil, fmt.Errorf("expected JSON object")
	}

	keys := []string{}
	for dec.More() {
		token, err = dec.Token()
		if err != nil {
			return nil, err
		}
		key, ok := token.(string)
		if !ok {
			return nil, fmt.Errorf("expected string key")
		}
		keys = append(keys, key)

		if err := skipJSONValue(dec); err != nil {
			return nil, err
		}
	}

	_, err = dec.Token()
	if err != nil {
		return nil, err
	}

	return keys, nil
}

func skipJSONValue(dec *json.Decoder) error {
	token, err := dec.Token()
	if err != nil {
		return err
	}
	return skipJSONValueAfterToken(dec, token)
}

func skipJSONValueAfterToken(dec *json.Decoder, token json.Token) error {
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}

	switch delim {
	case '{':
		for dec.More() {
			if _, err := dec.Token(); err != nil {
				return err
			}
			if err := skipJSONValue(dec); err != nil {
				return err
			}
		}
		_, err := dec.Token()
		return err
	case '[':
		for dec.More() {
			if err := skipJSONValue(dec); err != nil {
				return err
			}
		}
		_, err := dec.Token()
		return err
	default:
		return fmt.Errorf("unexpected delimiter %q", delim)
	}
}
