// Package utils - Media type checking utilities
package utils

// IsImageFile checks if file extension represents an image
func IsImageFile(ext string) bool {
	imageExts := []string{".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg", ".bmp", ".ico"}
	for _, imageExt := range imageExts {
		if ext == imageExt {
			return true
		}
	}
	return false
}

// IsVideoFile checks if file extension represents a video
func IsVideoFile(ext string) bool {
	videoExts := []string{".mp4", ".webm", ".ogg", ".avi", ".mov", ".wmv", ".flv", ".mkv"}
	for _, videoExt := range videoExts {
		if ext == videoExt {
			return true
		}
	}
	return false
}

// IsAudioFile checks if file extension represents audio
func IsAudioFile(ext string) bool {
	audioExts := []string{".mp3", ".wav", ".ogg", ".m4a", ".aac", ".flac", ".wma"}
	for _, audioExt := range audioExts {
		if ext == audioExt {
			return true
		}
	}
	return false
}

// GetFileTypeIcon returns appropriate Font Awesome icon for file type
func GetFileTypeIcon(ext string) string {
	switch {
	case IsImageFile(ext):
		return "fa-image"
	case IsVideoFile(ext):
		return "fa-video"
	case IsAudioFile(ext):
		return "fa-music"
	case ext == ".pdf":
		return "fa-file-pdf"
	case ext == ".doc" || ext == ".docx":
		return "fa-file-word"
	case ext == ".xls" || ext == ".xlsx":
		return "fa-file-excel"
	case ext == ".ppt" || ext == ".pptx":
		return "fa-file-powerpoint"
	case ext == ".txt":
		return "fa-file-alt"
	case ext == ".zip" || ext == ".rar" || ext == ".7z":
		return "fa-file-archive"
	default:
		return "fa-file"
	}
}
