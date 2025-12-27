package storage

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ResponseBody is a custom type that can store any content (JSON, HTML, text, etc.)
// It stores JSON as-is, and other content types as base64-encoded strings
type ResponseBody []byte

// MarshalJSON implements json.Marshaler for ResponseBody
func (rb ResponseBody) MarshalJSON() ([]byte, error) {
	// Try to parse as JSON - if it's valid JSON, return it as-is
	if len(rb) == 0 {
		return []byte("null"), nil
	}

	var jsonValue interface{}
	if err := json.Unmarshal(rb, &jsonValue); err == nil {
		// It's valid JSON, return it directly
		return json.Marshal(jsonValue)
	}

	// Not valid JSON, encode as base64 string
	encoded := base64.StdEncoding.EncodeToString(rb)
	return json.Marshal(encoded)
}

// UnmarshalJSON implements json.Unmarshaler for ResponseBody
func (rb *ResponseBody) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as a string (could be base64-encoded or plain string)
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		// Try to decode as base64 first
		decoded, err := base64.StdEncoding.DecodeString(str)
		if err == nil {
			// Successfully decoded from base64
			*rb = ResponseBody(decoded)
			return nil
		}
		// Not base64, treat as plain string
		*rb = ResponseBody(str)
		return nil
	}

	// Not a string, it's a JSON value (object, array, number, boolean, null)
	// Store the raw JSON bytes directly
	*rb = ResponseBody(data)
	return nil
}

// CachedResponse represents a cached HTTP response
type CachedResponse struct {
	Method     string              `json:"method"`
	Path       string              `json:"path"`
	StatusCode int                 `json:"status_code"`
	Headers    map[string][]string `json:"headers"`
	Body       ResponseBody        `json:"body"`
}

// Storage handles saving and loading cached responses
type Storage struct {
	basePath string
}

// New creates a new Storage instance
func New(basePath string) (*Storage, error) {
	// Create the storage directory if it doesn't exist
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	return &Storage{
		basePath: basePath,
	}, nil
}

// Exists checks if a cached response exists for the given hash
func (s *Storage) Exists(hash string) bool {
	filename := s.getFilename(hash)
	_, err := os.Stat(filename)
	return err == nil
}

// Load loads a cached response by hash
func (s *Storage) Load(hash string) (*CachedResponse, error) {
	filename := s.getFilename(hash)

	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read cached response: %w", err)
	}

	var cached CachedResponse
	if err := json.Unmarshal(data, &cached); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cached response: %w", err)
	}

	return &cached, nil
}

// Save saves a cached response using the hash as filename
func (s *Storage) Save(hash string, response *CachedResponse) error {
	filename := s.getFilename(hash)

	// Pretty print JSON with 2-space indentation
	data, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cached response: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write cached response: %w", err)
	}

	return nil
}

// getFilename returns the full file path for a given hash
func (s *Storage) getFilename(hash string) string {
	return filepath.Join(s.basePath, fmt.Sprintf("%s.json", hash))
}
