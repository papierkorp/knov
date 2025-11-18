// Package utils provides path utilities
package utils

// utils.Ternary helper function for conditional string selection
func Ternary(condition bool, trueVal, falseVal string) string {
	if condition {
		return trueVal
	}
	return falseVal
}
