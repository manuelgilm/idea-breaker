package sqlite

import (
	"encoding/json"
	"time"
)

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

func parseTime(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
}

func marshalTags(tags []string) string {
	if tags == nil {
		tags = []string{}
	}
	b, err := json.Marshal(tags)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func unmarshalTags(s string) ([]string, error) {
	tags := []string{}
	if err := json.Unmarshal([]byte(s), &tags); err != nil {
		return nil, err
	}
	return tags, nil
}
