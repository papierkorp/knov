package files

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"knov/internal/contentStorage"
	"knov/internal/logging"
	"knov/internal/parser"
	"knov/internal/utils"
)

var parserRegistry *parser.Registry

func init() {
	parserRegistry = parser.NewRegistry()
	parserRegistry.Register(parser.NewMarkdownHandler())
	parserRegistry.Register(parser.NewDokuwikiHandler())
	parserRegistry.Register(parser.NewPlaintextHandler())
}

// GetParserRegistry returns the global file type registry
func GetParserRegistry() *parser.Registry {
	return parserRegistry
}

// File represents a file in the system
type File struct {
	Name     string    `json:"name"`
	Path     string    `json:"path"`
	Metadata *Metadata `json:"metadata,omitempty"`
}

type FileContent struct {
	HTML string
	TOC  []TOCItem
}

// GetAllFiles returns list of all files
func GetAllFiles() ([]File, error) {
	docsDir := contentStorage.GetDocsPath()
	var files []File
	err := filepath.Walk(docsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}

		if !info.IsDir() {
			relativePath, err := filepath.Rel(docsDir, path)
			if err != nil {
				return err
			}
			file := File{
				Name: info.Name(),
				Path: relativePath,
			}
			files = append(files, file)
		}
		return nil
	})

	if err != nil {
		logging.LogError("failed to walk directory: %v", err)
		return nil, err
	}
	return files, nil
}

// GetAllMediaFiles returns list of all media files using contentStorage
func GetAllMediaFiles() ([]File, error) {
	mediaDir := contentStorage.GetMediaPath()
	var files []File

	// check if media directory exists
	if _, err := os.Stat(mediaDir); os.IsNotExist(err) {
		logging.LogDebug("media directory does not exist: %s", mediaDir)
		return files, nil // return empty slice, not error
	}

	err := filepath.Walk(mediaDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil // skip directories
		}

		relativePath, err := filepath.Rel(mediaDir, path)
		if err != nil {
			return err
		}

		// add media/ prefix to distinguish from docs files
		mediaPath := filepath.Join("media", relativePath)

		// get metadata if it exists
		metadata, _ := MetaDataGet(mediaPath)

		file := File{
			Name:     info.Name(),
			Path:     mediaPath,
			Metadata: metadata,
		}
		files = append(files, file)

		return nil
	})

	if err != nil {
		logging.LogError("failed to walk media directory: %v", err)
		return nil, err
	}

	logging.LogDebug("found %d media files", len(files))
	return files, nil
}

// GetFileContent converts file content to html based on detected type
func GetFileContent(filePath string) (*FileContent, error) {
	handler := parserRegistry.GetHandler(filePath)
	if handler == nil {
		return nil, fmt.Errorf("no handler found for file: %s", filePath)
	}

	content, err := handler.GetContent(filePath)
	if err != nil {
		return nil, err
	}

	parsed, err := handler.Parse(content)
	if err != nil {
		return nil, err
	}

	html, err := handler.Render(parsed, contentStorage.ToRelativePath(filePath))
	if err != nil {
		return nil, err
	}

	relativePath := contentStorage.ToRelativePath(filePath)
	processedContent := strings.ReplaceAll(string(html), "{{FILEPATH}}", relativePath)

	processedContent = InjectHeaderIDs(processedContent)
	toc := GenerateTOC(processedContent)

	return &FileContent{
		HTML: processedContent,
		TOC:  toc,
	}, nil
}

// GetAllFilesWithMetadata returns files with metadata
func GetAllFilesWithMetadata() ([]File, error) {
	files, err := GetAllFiles()
	if err != nil {
		return nil, err
	}

	for i := range files {
		files[i].Metadata = nil
	}

	return files, nil
}

// GetRawContent returns raw file content as string
func GetRawContent(filePath string) (string, error) {
	handler := parserRegistry.GetHandler(filePath)
	if handler == nil {
		return "", fmt.Errorf("no handler found for file: %s", filePath)
	}

	fullPath := contentStorage.ToDocsPath(filePath)
	content, err := handler.GetContent(fullPath)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

// ExtractSectionContent extracts content of a specific section from a file
func ExtractSectionContent(filePath, sectionID string) (string, error) {
	handler := parserRegistry.GetHandler(filePath)
	if handler == nil {
		return "", fmt.Errorf("no handler found for file: %s", filePath)
	}

	// check if this is a markdown file
	if _, ok := handler.(*parser.MarkdownHandler); !ok {
		return "", fmt.Errorf("section editing not supported for file type: %s", filePath)
	}

	fullPath := contentStorage.ToDocsPath(filePath)
	content, err := handler.GetContent(fullPath)
	if err != nil {
		return "", err
	}

	return extractSectionFromMarkdown(string(content), sectionID)
}

// SaveSectionContent saves content to a specific section in a file
func SaveSectionContent(filePath, sectionID, sectionContent string) error {
	handler := parserRegistry.GetHandler(filePath)
	if handler == nil {
		return fmt.Errorf("no handler found for file: %s", filePath)
	}

	// check if this is a markdown file
	if _, ok := handler.(*parser.MarkdownHandler); !ok {
		return fmt.Errorf("section editing not supported for file type: %s", filePath)
	}

	fullPath := contentStorage.ToDocsPath(filePath)
	originalContent, err := handler.GetContent(fullPath)
	if err != nil {
		return err
	}

	updatedContent, err := replaceSectionInMarkdown(string(originalContent), sectionID, sectionContent)
	if err != nil {
		return err
	}

	return SaveRawContent(fullPath, updatedContent)
}

// extractSectionFromMarkdown extracts content between headers including the header itself
func extractSectionFromMarkdown(content, sectionID string) (string, error) {
	lines := strings.Split(content, "\n")

	var sectionStart, sectionEnd int
	var inSection bool
	var inCodeBlock bool
	usedIDs := make(map[string]int)

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// check for code block fences
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
		}

		// only process headers outside of code blocks
		if !inCodeBlock && strings.HasPrefix(trimmed, "#") {
			headerText := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
			headerID := utils.GenerateID(headerText, usedIDs)

			if headerID == sectionID && !inSection {
				sectionStart = i
				inSection = true
				continue
			}

			if inSection && headerID != sectionID {
				sectionEnd = i
				break
			}
		}
	}

	if !inSection {
		return "", fmt.Errorf("section not found: %s", sectionID)
	}

	if sectionEnd == 0 {
		sectionEnd = len(lines)
	}

	sectionLines := lines[sectionStart:sectionEnd]
	return strings.Join(sectionLines, "\n"), nil
}

// replaceSectionInMarkdown replaces content of a specific section including the header
func replaceSectionInMarkdown(content, sectionID, newContent string) (string, error) {
	lines := strings.Split(content, "\n")

	var result []string
	var inTargetSection bool
	var inCodeBlock bool
	var headerLevel int
	usedIDs := make(map[string]int)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// check for code block fences
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
		}

		// only process headers outside of code blocks
		if !inCodeBlock && strings.HasPrefix(trimmed, "#") {
			level := len(trimmed) - len(strings.TrimLeft(trimmed, "#"))
			headerText := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
			headerID := utils.GenerateID(headerText, usedIDs)

			if headerID == sectionID && !inTargetSection {
				// start of target section - replace with new content
				inTargetSection = true
				headerLevel = level
				if strings.TrimSpace(newContent) != "" {
					result = append(result, strings.Split(newContent, "\n")...)
				}
				continue
			} else if inTargetSection && level <= headerLevel {
				// reached next section of same or higher level
				inTargetSection = false
				result = append(result, line)
				continue
			}
		}

		if !inTargetSection {
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n"), nil
}

// SaveRawContent saves raw content to file (creates if not exists)
func SaveRawContent(filePath string, content string) error {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		logging.LogError("failed to create directory %s: %v", dir, err)
		return err
	}

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		logging.LogError("failed to write file %s: %v", filePath, err)
		return err
	}

	logging.LogInfo("saved file: %s", filePath)
	return nil
}
