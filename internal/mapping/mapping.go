package mapping

// URLToDatabase maps URL-friendly field names (singular) to database field names (plural/actual)
func URLToDatabase(urlField string) string {
	switch urlField {
	case "tag":
		return "tags"
	case "folder":
		return "folders"
	case "projects":
		return "para_projects"
	case "areas":
		return "para_areas"
	case "resources":
		return "para_resources"
	case "archive":
		return "para_archive"
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
	case "para_projects":
		return "projects"
	case "para_areas":
		return "areas"
	case "para_resources":
		return "resources"
	case "para_archive":
		return "archive"
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
	case "para_projects":
		return "projects"
	case "para_areas":
		return "areas"
	case "para_resources":
		return "resources"
	case "para_archive":
		return "archive"
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
	case "projects", "para_projects":
		return true
	case "areas", "para_areas":
		return true
	case "resources", "para_resources":
		return true
	case "archive", "para_archive":
		return true
	default:
		return false
	}
}
