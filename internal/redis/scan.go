package redis

import "fmt"

// parseScanResult parses a SCAN response into (nextCursor, keys).
// Returns an error if the response shape is unexpected.
func parseScanResult(result interface{}) (nextCursor string, keys []string, err error) {
	arr, ok := result.([]interface{})
	if !ok || len(arr) != 2 {
		return "", nil, fmt.Errorf("redis: unexpected SCAN result type")
	}
	nextCursor, ok = arr[0].(string)
	if !ok {
		return "", nil, fmt.Errorf("redis: unexpected SCAN cursor type")
	}
	rawKeys, ok := arr[1].([]interface{})
	if !ok {
		return "", nil, fmt.Errorf("redis: unexpected SCAN keys type")
	}
	keys = make([]string, 0, len(rawKeys))
	for _, k := range rawKeys {
		if s, strOk := k.(string); strOk {
			keys = append(keys, s)
		}
	}
	return nextCursor, keys, nil
}

// doPing sends a PING and verifies the response.
func doPing(conn Doer) error {
	val, err := String(conn.Do("PING"))
	if err != nil {
		return err
	}
	if val != pong {
		return fmt.Errorf("redis: unexpected PING response: %q", val)
	}
	return nil
}
