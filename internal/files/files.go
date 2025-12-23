package files

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"knov/internal/configmanager"
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
	dataDir := configmanager.GetAppConfig().DataPath
	var files []File
	err := filepath.Walk(dataDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}

		if !info.IsDir() {
			relativePath, err := filepath.Rel(dataDir, path)
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

	html, err := handler.Render(parsed, utils.ToRelativePath(filePath))
	if err != nil {
		return nil, err
	}

	relativePath := utils.ToRelativePath(filePath)
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

	fullPath := utils.ToFullPath(filePath)
	content, err := handler.GetContent(fullPath)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

// ExtractSectionContent extracts content of a specific section from markdown
func ExtractSectionContent(filePath, sectionID string) (string, error) {
	content, err := GetRawContent(filePath)
	if err != nil {
		return "", err
	}

	return extractSectionFromMarkdown(content, sectionID), nil
}

// SaveSectionContent saves content to a specific section in a markdown file
func SaveSectionContent(filePath, sectionID, sectionContent string) error {
	originalContent, err := GetRawContent(filePath)
	if err != nil {
		return err
	}

	updatedContent := replaceSectionInMarkdown(originalContent, sectionID, sectionContent)
	fullPath := utils.ToFullPath(filePath)
	return SaveRawContent(fullPath, updatedContent)
}

// extractSectionFromMarkdown extracts content between headers including the header itself
func extractSectionFromMarkdown(content, sectionID string) string {
	lines := strings.Split(content, "\n")
	usedIDs := make(map[string]int)
	var sectionLines []string
	inTargetSection := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// check if this is a header line
		if strings.HasPrefix(trimmed, "#") {
			// extract header text and generate ID
			headerText := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
			headerID := utils.GenerateID(headerText, usedIDs)

			if headerID == sectionID && !inTargetSection {
				// start of our target section - include the header
				inTargetSection = true
				sectionLines = append(sectionLines, line)
				continue
			} else if inTargetSection && headerID != sectionID {
				// reached next section, stop
				break
			}
		}

		if inTargetSection {
			sectionLines = append(sectionLines, line)
		}
	}

	return strings.Join(sectionLines, "\n")
}

// replaceSectionInMarkdown replaces content of a specific section including the header
func replaceSectionInMarkdown(content, sectionID, newContent string) string {
	lines := strings.Split(content, "\n")
	usedIDs := make(map[string]int)
	var result []string
	inTargetSection := false
	headerLevel := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// check if this is a header line
		if strings.HasPrefix(trimmed, "#") {
			level := len(trimmed) - len(strings.TrimLeft(trimmed, "#"))
			headerText := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
			headerID := utils.GenerateID(headerText, usedIDs)

			if headerID == sectionID && !inTargetSection {
				// start of our target section - replace with new content
				inTargetSection = true
				headerLevel = level
				// add new content (which should include the header)
				if strings.TrimSpace(newContent) != "" {
					result = append(result, strings.Split(newContent, "\n")...)
				}
				continue
			} else if inTargetSection && level <= headerLevel {
				// reached next section of same or higher level, stop replacing
				inTargetSection = false
				result = append(result, line)
				continue
			}
		}

		if !inTargetSection {
			result = append(result, line)
		}
		// if inTargetSection is true, we skip the line (replacement)
	}

	return strings.Join(result, "\n")
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
