package util

import (
	"encoding/json"
	"strings"
)

func IsJSONString(s string) bool {
	var js string
	return json.Unmarshal([]byte(s), &js) == nil
}

func IsJSON(s string) bool {
	var js map[string]interface{}
	return json.Unmarshal([]byte(s), &js) == nil
}

func SplitAny(s string, seps string) []string {
	splitter := func(r rune) bool {
		return strings.ContainsRune(seps, r)
	}
	return strings.FieldsFunc(s, splitter)
}
