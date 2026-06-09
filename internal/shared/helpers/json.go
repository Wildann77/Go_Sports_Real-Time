package helpers

import "encoding/json"

func IsValidJSON(s string) bool {
	var js map[string]interface{}
	return json.Unmarshal([]byte(s), &js) == nil
}

func IsValidJSONBytes(b []byte) bool {
	var js map[string]interface{}
	return json.Unmarshal(b, &js) == nil
}
