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
