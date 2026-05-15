package redis

import "encoding/json"

// mustMarshalJSON marshals v to a JSON string. It panics on failure.
// This is used for Go structs with only JSON-safe field types, where
// json.Marshal cannot fail.
func mustMarshalJSON(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		panic("redis: marshal: " + err.Error())
	}
	return string(data)
}
