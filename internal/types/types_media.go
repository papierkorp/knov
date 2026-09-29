package types

import (
	"mime"
	"path/filepath"
	"strings"
)

// media categories returned by MediaCategory
const (
	MediaCategoryImage    = "image"
	MediaCategoryVideo    = "video"
	MediaCategoryAudio    = "audio"
	MediaCategoryDocument = "document"
	MediaCategoryArchive  = "archive"
	MediaCategoryText     = "text"
	MediaCategoryFont     = "font"
	MediaCategoryProgram  = "program"
	MediaCategoryOther    = "other"
)

type mediaType struct {
	mime     string // empty when there is no sensible mime type, served as application/octet-stream
	category string
}

// mediaTypes is the fixed extension table used for both mime types and categories, since
// mime.TypeByExtension depends on the host (/etc/mime.types, windows registry)
var mediaTypes = map[string]mediaType{
	".png": {"image/png", MediaCategoryImage}, ".jpg": {"image/jpeg", MediaCategoryImage}, ".jpeg": {"image/jpeg", MediaCategoryImage},
	".gif": {"image/gif", MediaCategoryImage}, ".webp": {"image/webp", MediaCategoryImage}, ".svg": {"image/svg+xml", MediaCategoryImage},
	".bmp": {"image/bmp", MediaCategoryImage}, ".ico": {"image/vnd.microsoft.icon", MediaCategoryImage},
	".tif": {"image/tiff", MediaCategoryImage}, ".tiff": {"image/tiff", MediaCategoryImage}, ".avif": {"image/avif", MediaCategoryImage},
	".heic": {"image/heic", MediaCategoryImage}, ".heif": {"image/heif", MediaCategoryImage},

	".mp4": {"video/mp4", MediaCategoryVideo}, ".webm": {"video/webm", MediaCategoryVideo}, ".mkv": {"video/x-matroska", MediaCategoryVideo},
	".mov": {"video/quicktime", MediaCategoryVideo}, ".avi": {"video/x-msvideo", MediaCategoryVideo}, ".wmv": {"video/x-ms-wmv", MediaCategoryVideo},
	".flv": {"video/x-flv", MediaCategoryVideo}, ".m4v": {"video/x-m4v", MediaCategoryVideo}, ".mpg": {"video/mpeg", MediaCategoryVideo},
	".mpeg": {"video/mpeg", MediaCategoryVideo}, ".ogv": {"video/ogg", MediaCategoryVideo}, ".3gp": {"video/3gpp", MediaCategoryVideo},

	".mp3": {"audio/mpeg", MediaCategoryAudio}, ".wav": {"audio/wav", MediaCategoryAudio}, ".ogg": {"audio/ogg", MediaCategoryAudio},
	".oga": {"audio/ogg", MediaCategoryAudio}, ".flac": {"audio/flac", MediaCategoryAudio}, ".aac": {"audio/aac", MediaCategoryAudio},
	".m4a": {"audio/mp4", MediaCategoryAudio}, ".opus": {"audio/opus", MediaCategoryAudio}, ".wma": {"audio/x-ms-wma", MediaCategoryAudio},
	".mid": {"audio/midi", MediaCategoryAudio}, ".midi": {"audio/midi", MediaCategoryAudio},

	".pdf": {"application/pdf", MediaCategoryDocument}, ".epub": {"application/epub+zip", MediaCategoryDocument},
	".rtf": {"application/rtf", MediaCategoryDocument}, ".doc": {"application/msword", MediaCategoryDocument},
	".docx": {"application/vnd.openxmlformats-officedocument.wordprocessingml.document", MediaCategoryDocument},
	".xls":  {"application/vnd.ms-excel", MediaCategoryDocument},
	".xlsx": {"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", MediaCategoryDocument},
	".ppt":  {"application/vnd.ms-powerpoint", MediaCategoryDocument},
	".pptx": {"application/vnd.openxmlformats-officedocument.presentationml.presentation", MediaCategoryDocument},
	".odt":  {"application/vnd.oasis.opendocument.text", MediaCategoryDocument},
	".ods":  {"application/vnd.oasis.opendocument.spreadsheet", MediaCategoryDocument},
	".odp":  {"application/vnd.oasis.opendocument.presentation", MediaCategoryDocument},
	".odg":  {"application/vnd.oasis.opendocument.graphics", MediaCategoryDocument},

	".zip": {"application/zip", MediaCategoryArchive}, ".rar": {"application/vnd.rar", MediaCategoryArchive},
	".7z": {"application/x-7z-compressed", MediaCategoryArchive}, ".gz": {"application/gzip", MediaCategoryArchive},
	".tgz": {"application/gzip", MediaCategoryArchive}, ".tar": {"application/x-tar", MediaCategoryArchive},
	".bz2": {"application/x-bzip2", MediaCategoryArchive}, ".xz": {"application/x-xz", MediaCategoryArchive},
	".zst": {"application/zstd", MediaCategoryArchive},

	".txt": {"text/plain", MediaCategoryText}, ".csv": {"text/csv", MediaCategoryText}, ".json": {"application/json", MediaCategoryText},
	".xml": {"text/xml", MediaCategoryText}, ".yaml": {"application/yaml", MediaCategoryText}, ".yml": {"application/yaml", MediaCategoryText},
	".toml": {"application/toml", MediaCategoryText}, ".html": {"text/html", MediaCategoryText}, ".htm": {"text/html", MediaCategoryText},
	".css": {"text/css", MediaCategoryText}, ".js": {"text/javascript", MediaCategoryText}, ".excalidraw": {"", MediaCategoryText},
	".md": {"text/markdown", MediaCategoryText}, ".vtt": {"text/vtt", MediaCategoryText}, ".log": {"text/plain", MediaCategoryText},
	".ini": {"text/plain", MediaCategoryText}, ".conf": {"text/plain", MediaCategoryText}, ".cfg": {"text/plain", MediaCategoryText},
	".sh": {"text/plain", MediaCategoryText}, ".bat": {"text/plain", MediaCategoryText}, ".cmd": {"text/plain", MediaCategoryText},
	".ps1": {"text/plain", MediaCategoryText}, ".py": {"text/plain", MediaCategoryText}, ".rb": {"text/plain", MediaCategoryText},
	".pl": {"text/plain", MediaCategoryText}, ".go": {"text/plain", MediaCategoryText}, ".ts": {"text/plain", MediaCategoryText},
	".sql": {"text/plain", MediaCategoryText},

	".ttf": {"font/ttf", MediaCategoryFont}, ".otf": {"font/otf", MediaCategoryFont},
	".woff": {"font/woff", MediaCategoryFont}, ".woff2": {"font/woff2", MediaCategoryFont},

	".exe": {"application/vnd.microsoft.portable-executable", MediaCategoryProgram},
	".dll": {"application/vnd.microsoft.portable-executable", MediaCategoryProgram},
	".msi": {"application/x-msi", MediaCategoryProgram}, ".deb": {"application/vnd.debian.binary-package", MediaCategoryProgram},
	".rpm": {"application/x-rpm", MediaCategoryProgram}, ".apk": {"application/vnd.android.package-archive", MediaCategoryProgram},
	".dmg": {"application/x-apple-diskimage", MediaCategoryProgram}, ".pkg": {"", MediaCategoryProgram},
	".appimage": {"", MediaCategoryProgram}, ".jar": {"application/java-archive", MediaCategoryProgram},
}

// MimeTypeByExtension returns the clean lowercase mime type for an extension (no parameters) - from the
// fixed table, falling back to the host's mime table only for extensions not listed there.
// empty if unknown.
func MimeTypeByExtension(ext string) string {
	ext = strings.ToLower(ext)
	if t, ok := mediaTypes[ext]; ok {
		return t.mime
	}
	mimeType := mime.TypeByExtension(ext)
	if i := strings.Index(mimeType, ";"); i >= 0 {
		mimeType = strings.TrimSpace(mimeType[:i])
	}
	return strings.ToLower(mimeType)
}

// ServeMimeType returns the mime type to serve or display an extension with -
// application/octet-stream if unknown.
func ServeMimeType(ext string) string {
	if mimeType := MimeTypeByExtension(ext); mimeType != "" {
		return mimeType
	}
	return "application/octet-stream"
}

// MediaCategory returns the category of path (or a bare extension like ".png") by its
// extension, or MediaCategoryOther if unknown.
func MediaCategory(path string) string {
	if t, ok := mediaTypes[strings.ToLower(filepath.Ext(path))]; ok {
		return t.category
	}
	return MediaCategoryOther
}
