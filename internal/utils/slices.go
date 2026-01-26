package utils

import "slices"

// setToSortedSlice converts a map[string]bool to a sorted slice
func SetToSortedSlice(set map[string]bool) []string {
	var slice []string
	for key := range set {
		slice = append(slice, key)
	}
	slices.Sort(slice)
	return slice
}
