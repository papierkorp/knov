// Package chat handles the chat/stream-of-consciousness feature
package chat

import (
	"knov/internal/chatStorage"
)

const PageSize = 50

// Message is re-exported from chatStorage for use by render/api layers
type Message = chatStorage.Message

// Add creates a new message
func Add(content, filePath string) (*Message, error) {
	return chatStorage.Add(content, filePath)
}

// Delete removes a message by ID
func Delete(id string) error {
	return chatStorage.Delete(id)
}

// GetByID returns a single message by ID
func GetByID(id string) (*Message, error) {
	return chatStorage.GetByID(id)
}

// GetPage returns paginated messages and total count
func GetPage(filePath string, offset int) ([]Message, int, error) {
	return chatStorage.GetPage(filePath, PageSize, offset)
}

// MoveFilePath reattaches all messages from oldPath to newPath (used when a file is renamed/moved)
func MoveFilePath(oldPath, newPath string) error {
	return chatStorage.MoveFilePath(oldPath, newPath)
}

// DeleteForFile removes all messages attached to the given file path (used when a file is deleted)
func DeleteForFile(filePath string) error {
	return chatStorage.DeleteByFilePath(filePath)
}
