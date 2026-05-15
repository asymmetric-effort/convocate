package dispatch

import "encoding/json"

// mustMarshalJSON marshals v to JSON bytes. It panics on failure.
// Used for Go structs with only JSON-safe field types, where
// json.Marshal cannot fail.
func mustMarshalJSON(v interface{}) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic("dispatch: marshal: " + err.Error())
	}
	return data
}
