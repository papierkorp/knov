// Package types contains shared data structures used across packages
package types

// Criteria represents a single filter condition
type Criteria struct {
	Metadata string `json:"metadata"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
	Action   string `json:"action"`
}
