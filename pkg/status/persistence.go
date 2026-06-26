package status

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

func WriteStatus(s *SessionStatus) error {
	s.LastUpdated = time.Now().Format(time.RFC3339)
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	path := ResolveStatusPath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		path = filepath.Join("/tmp/remote-studio", "status.json")
		dir = filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	// Write atomically using temporary file in same directory
	tmpFile, err := os.CreateTemp(dir, "status-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return err
	}
	if err := tmpFile.Sync(); err != nil {
		tmpFile.Close()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpFile.Name(), 0644); err != nil {
		return err
	}
	return os.Rename(tmpFile.Name(), path)
}

func ReadStatus() (*SessionStatus, error) {
	path := ResolveStatusPath()
	data, err := os.ReadFile(path)
	if err != nil {
		fallbackPath := filepath.Join("/tmp/remote-studio", "status.json")
		data, err = os.ReadFile(fallbackPath)
		if err != nil {
			return nil, err
		}
	}

	var s SessionStatus
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}
