package sqlite

import (
	"encoding/json"
	"log"
)

func mustJSONArray(value []string) string {
	if value == nil {
		return "[]"
	}

	data, err := json.Marshal(value)
	if err != nil {
		log.Fatalf("marshal json array: %v", err)
	}

	if string(data) == "null" {
		return "[]"
	}

	return string(data)
}
