package utils

import "encoding/json"

// StripJSONKeys removes the given top-level keys from a JSON object. It returns
// raw unchanged when it is empty, not a JSON object, or cannot be parsed.
func StripJSONKeys(raw json.RawMessage, keys ...string) json.RawMessage {
	if len(raw) == 0 || len(keys) == 0 {
		return raw
	}
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		return raw
	}
	removed := false
	for _, key := range keys {
		if _, ok := data[key]; ok {
			delete(data, key)
			removed = true
		}
	}
	if !removed {
		return raw
	}
	stripped, err := json.Marshal(data)
	if err != nil {
		return raw
	}
	return stripped
}
