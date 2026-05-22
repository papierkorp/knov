package mapping

// URLToDatabase maps URL-friendly field names (singular) to database field names (plural/actual)
func URLToDatabase(urlField string) string {
	switch urlField {
	case "tag":
		return "tags"
	case "folder":
		return "folders"
	default:
		return urlField
	}
}

// DatabaseToURL maps database field names to URL-friendly field names (for browse links)
func DatabaseToURL(dbField string) string {
	switch dbField {
	case "tags":
		return "tag"
	case "folders":
		return "folder"
	default:
		return dbField
	}
}

// GetDisplayName returns user-friendly display name for database field names
func GetDisplayName(dbField string) string {
	switch dbField {
	case "tags":
		return "tags"
	case "folders":
		return "folders"
	case "child-of":
		return "child of"
	case "parent-of":
		return "parent of"
	case "ancestor-of":
		return "ancestor of"
	default:
		return dbField
	}
}

// IsArrayField determines if a field is an array type (uses "contains" operator)
func IsArrayField(field string) bool {
	switch field {
	case "tag", "tags":
		return true
	case "folder", "folders":
		return true
	default:
		return false
	}
}
