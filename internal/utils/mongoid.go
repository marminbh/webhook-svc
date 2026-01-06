package utils

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// MongoDBObjectIDLength is the length of a MongoDB ObjectID in hex characters
const MongoDBObjectIDLength = 24

// UUIDLength is the length of a UUID in hex characters (without dashes)
const UUIDLength = 32

// ConvertMongoIDToUUID converts a MongoDB ObjectID (24 hex characters) to a UUID
// by prepending zeros to make it 32 characters, then formatting as a UUID
// Example: "682c5990bf4a775c8de9598a" -> "00000000-682c-5990-bf4a-775c8de9598a"
func ConvertMongoIDToUUID(mongoID string) (uuid.UUID, error) {
	// Remove any whitespace
	mongoID = strings.TrimSpace(mongoID)

	// Validate MongoDB ObjectID length (24 hex characters)
	if len(mongoID) != MongoDBObjectIDLength {
		return uuid.Nil, fmt.Errorf("invalid MongoDB ObjectID length: expected %d characters, got %d", MongoDBObjectIDLength, len(mongoID))
	}

	// Validate that it's a valid hex string
	for _, char := range mongoID {
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F')) {
			return uuid.Nil, fmt.Errorf("invalid MongoDB ObjectID: contains non-hexadecimal characters")
		}
	}

	// Prepend zeros to make it 32 characters (UUID length)
	// UUID format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx (32 hex chars total)
	// We need 8 zeros at the start: 00000000 + mongoID
	paddedID := strings.Repeat("0", UUIDLength-MongoDBObjectIDLength) + mongoID

	// Format as UUID: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	// Split: 8-4-4-4-12
	uuidStr := fmt.Sprintf("%s-%s-%s-%s-%s",
		paddedID[0:8],   // 8 chars
		paddedID[8:12],  // 4 chars
		paddedID[12:16], // 4 chars
		paddedID[16:20], // 4 chars
		paddedID[20:32], // 12 chars
	)

	// Parse and return UUID
	return uuid.Parse(uuidStr)
}
