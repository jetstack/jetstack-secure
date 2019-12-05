// Package api provides types for Preflight reports and some common helpers.
package api

import (
	"encoding/json"
	"time"
)

// TimeFormat defines the format used for timestamps across all this API.
const TimeFormat = time.RFC3339

// Time is a wrapper around time.Time that overrides how it is marshaled into JSON
type Time struct {
	time.Time
}

// String returns a string representation of the timestamp
func (t Time) String() string {
	return t.Format(TimeFormat)
}

// MarshalJSON marshals the timestamp with RFC3339 format
func (t Time) MarshalJSON() ([]byte, error) {
	str := t.String()
	jsonStr, err := json.Marshal(str)
	if err != nil {
		return nil, err
	}
	return []byte(jsonStr), nil
}
