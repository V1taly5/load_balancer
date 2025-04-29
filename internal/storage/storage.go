package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

type BucketState struct {
	Tokens     float64   `json:"tokens"`
	Capacity   float64   `json:"capacity"`
	Rate       float64   `json:"rate"`
	LastUpdate time.Time `json:"last_update"`
}

type FileStorage struct {
	filePath string
	mu       sync.Mutex
}

func NewFileStorage(filePath string) (*FileStorage, error) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			file, createErr := os.Create(filePath)
			if createErr != nil {
				return nil, fmt.Errorf("failed to create file: %w", createErr)
			}
			defer file.Close()

			_, writeErr := file.Write([]byte("{}"))
			if writeErr != nil {
				return nil, fmt.Errorf("failed to write initial JSON to file: %w", writeErr)
			}

			fileInfo, err = os.Stat(filePath)
			if err != nil {
				return nil, fmt.Errorf("failed to stat newly created file: %w", err)
			}
		} else {
			return nil, fmt.Errorf("file verification error: %w", err)
		}
	}
	if fileInfo.IsDir() {
		return nil, fmt.Errorf("path is a directory, not a file")
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	if fileInfo.Size() != 0 {
		var jsonData interface{}
		if err := json.Unmarshal(data, &jsonData); err != nil {
			return nil, fmt.Errorf("invalid JSON in file: %w", err)
		}
	}

	return &FileStorage{filePath: filePath}, nil
}

// Cохраняет состояние клиента
func (fs *FileStorage) Save(clientID string, state *BucketState) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	data, err := os.ReadFile(fs.filePath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	clients := make(map[string]*BucketState)
	if len(data) > 0 {
		if err := json.Unmarshal(data, &clients); err != nil {
			return err
		}
	}

	clients[clientID] = state
	newData, err := json.MarshalIndent(clients, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(fs.filePath, newData, 0644)
}

// Загружает все состояния клиентов
func (fs *FileStorage) LoadAll() (map[string]*BucketState, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	data, err := os.ReadFile(fs.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	clients := make(map[string]*BucketState)
	if err := json.Unmarshal(data, &clients); err != nil {
		return nil, err
	}

	return clients, nil
}

// Удаляет состояние клиента
func (fs *FileStorage) Delete(clientID string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	data, err := os.ReadFile(fs.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	clients := make(map[string]*BucketState)
	if len(data) > 0 {
		if err := json.Unmarshal(data, &clients); err != nil {
			return err
		}
	}

	delete(clients, clientID)
	newData, err := json.MarshalIndent(clients, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(fs.filePath, newData, 0644)
}
