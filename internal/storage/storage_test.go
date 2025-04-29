package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTempFile(t *testing.T) string {
	file, err := os.CreateTemp("", "filestorage_test_*.json")
	require.NoError(t, err)
	file.Close()
	t.Cleanup(func() { os.Remove(file.Name()) })
	return file.Name()
}

// Создает временный файл с передоваемым содержимым
func createFileWithContent(t *testing.T, content []byte) string {
	filename := createTempFile(t)
	err := os.WriteFile(filename, content, 0644)
	require.NoError(t, err)
	return filename
}

// Создает файл с предопределенным состоянием клиентов
func createFileWithState(t *testing.T, clients map[string]*BucketState) string {
	data, err := json.MarshalIndent(clients, "", "  ")
	require.NoError(t, err)
	return createFileWithContent(t, data)
}

func TestFileStorage(t *testing.T) {
	t.Run("NewFileStorage", func(t *testing.T) {
		t.Run("File does not exist", func(t *testing.T) {
			_, err := NewFileStorage("non_existent_file.json")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "file not found")
		})

		t.Run("File exists but is empty", func(t *testing.T) {
			tempFile := createFileWithContent(t, []byte{})
			_, err := NewFileStorage(tempFile)
			require.NoError(t, err)
		})

		t.Run("File exists with valid JSON", func(t *testing.T) {
			tempFile := createFileWithContent(t, []byte("{}"))
			fs, err := NewFileStorage(tempFile)
			require.NoError(t, err)
			assert.Equal(t, tempFile, fs.filePath)
		})

		t.Run("File exists with invalid JSON", func(t *testing.T) {
			tempFile := createFileWithContent(t, []byte("{invalid json"))
			_, err := NewFileStorage(tempFile)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid character")
		})
	})

	t.Run("Save operations", func(t *testing.T) {
		t.Run("Save client to empty storage", func(t *testing.T) {
			tempFile := createFileWithContent(t, []byte("{}"))
			fs, err := NewFileStorage(tempFile)
			require.NoError(t, err)

			state := &BucketState{
				Tokens:     5,
				Capacity:   10,
				Rate:       1,
				LastUpdate: time.Now().UTC().Truncate(time.Second),
			}

			err = fs.Save("client1", state)
			require.NoError(t, err)

			data, err := os.ReadFile(tempFile)
			require.NoError(t, err)

			var savedStates map[string]*BucketState
			err = json.Unmarshal(data, &savedStates)
			require.NoError(t, err)
			require.Contains(t, savedStates, "client1")

			savedState := savedStates["client1"]
			assert.InDelta(t, state.Tokens, savedState.Tokens, 0.001)
			assert.InDelta(t, state.Capacity, savedState.Capacity, 0.001)
			assert.InDelta(t, state.Rate, savedState.Rate, 0.001)
			assert.Equal(t, state.LastUpdate.Unix(), savedState.LastUpdate.Unix())
		})

		t.Run("Save and overwrite existing client", func(t *testing.T) {
			initialState := map[string]*BucketState{
				"client1": {
					Tokens:     10,
					Capacity:   20,
					Rate:       2,
					LastUpdate: time.Now().UTC().Add(-time.Hour),
				},
			}
			tempFile := createFileWithState(t, initialState)

			fs, err := NewFileStorage(tempFile)
			require.NoError(t, err)

			newState := &BucketState{
				Tokens:     5,
				Capacity:   10,
				Rate:       1,
				LastUpdate: time.Now().UTC().Truncate(time.Second),
			}

			// перезаписываем существующего клиента
			err = fs.Save("client1", newState)
			require.NoError(t, err)

			clients, err := fs.LoadAll()
			require.NoError(t, err)
			require.Contains(t, clients, "client1")

			// проверяем что данные были заменены
			updatedState := clients["client1"]
			assert.InDelta(t, newState.Tokens, updatedState.Tokens, 0.001)
			assert.InDelta(t, newState.Capacity, updatedState.Capacity, 0.001)
			assert.InDelta(t, newState.Rate, updatedState.Rate, 0.001)
			assert.Equal(t, newState.LastUpdate.Unix(), updatedState.LastUpdate.Unix())
		})

		t.Run("Save multiple clients", func(t *testing.T) {
			tempFile := createFileWithContent(t, []byte("{}"))
			fs, err := NewFileStorage(tempFile)
			require.NoError(t, err)

			client1 := &BucketState{
				Tokens:     5,
				Capacity:   10,
				Rate:       1,
				LastUpdate: time.Now().UTC().Truncate(time.Second),
			}

			client2 := &BucketState{
				Tokens:     15,
				Capacity:   20,
				Rate:       2,
				LastUpdate: time.Now().UTC().Truncate(time.Second),
			}

			err = fs.Save("client1", client1)
			require.NoError(t, err)

			err = fs.Save("client2", client2)
			require.NoError(t, err)

			clients, err := fs.LoadAll()
			require.NoError(t, err)
			assert.Len(t, clients, 2)
			assert.Contains(t, clients, "client1")
			assert.Contains(t, clients, "client2")
		})
	})

	t.Run("LoadAll operations", func(t *testing.T) {
		t.Run("LoadAll with empty file", func(t *testing.T) {
			tempFile := createFileWithContent(t, []byte("{}"))
			fs, err := NewFileStorage(tempFile)
			require.NoError(t, err)

			clients, err := fs.LoadAll()
			require.NoError(t, err)
			assert.Empty(t, clients)
		})

		t.Run("LoadAll with existing clients", func(t *testing.T) {
			now := time.Now().UTC().Truncate(time.Second)
			initialState := map[string]*BucketState{
				"client1": {
					Tokens:     10,
					Capacity:   20,
					Rate:       2,
					LastUpdate: now,
				},
				"client2": {
					Tokens:     5,
					Capacity:   10,
					Rate:       1,
					LastUpdate: now,
				},
			}
			tempFile := createFileWithState(t, initialState)

			fs, err := NewFileStorage(tempFile)
			require.NoError(t, err)

			clients, err := fs.LoadAll()
			require.NoError(t, err)
			assert.Len(t, clients, 2)

			assert.Contains(t, clients, "client1")
			assert.InDelta(t, initialState["client1"].Tokens, clients["client1"].Tokens, 0.001)
			assert.InDelta(t, initialState["client1"].Capacity, clients["client1"].Capacity, 0.001)
			assert.InDelta(t, initialState["client1"].Rate, clients["client1"].Rate, 0.001)
			assert.Equal(t, initialState["client1"].LastUpdate.Unix(), clients["client1"].LastUpdate.Unix())

			assert.Contains(t, clients, "client2")
			assert.InDelta(t, initialState["client2"].Tokens, clients["client2"].Tokens, 0.001)
			assert.InDelta(t, initialState["client2"].Capacity, clients["client2"].Capacity, 0.001)
			assert.InDelta(t, initialState["client2"].Rate, clients["client2"].Rate, 0.001)
			assert.Equal(t, initialState["client2"].LastUpdate.Unix(), clients["client2"].LastUpdate.Unix())
		})

		t.Run("LoadAll with corrupted file", func(t *testing.T) {
			tempFile := createFileWithContent(t, []byte("{bad json"))
			fs, err := NewFileStorage(tempFile)
			require.Error(t, err)

			require.Panics(t, func() {
				_, _ = fs.LoadAll()
			}, "LoadAll was expected to panic")
			require.Error(t, err)
		})

		t.Run("LoadAll on file with permission issues", func(t *testing.T) {
			if os.Getuid() == 0 {
				t.Skip("Skipping test when running as root")
			}

			tempFile := createFileWithContent(t, []byte("{}"))
			fs, err := NewFileStorage(tempFile)
			require.NoError(t, err)

			// меняем права доступа чтобы вызвать ошибку при чтении
			err = os.Chmod(tempFile, 0000)
			require.NoError(t, err)
			t.Cleanup(func() { os.Chmod(tempFile, 0644) })

			_, err = fs.LoadAll()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "permission denied")
		})
	})

	t.Run("Concurrency safety", func(t *testing.T) {
		tempFile := createFileWithContent(t, []byte("{}"))
		fs, err := NewFileStorage(tempFile)
		require.NoError(t, err)

		// создаем несколько горутин для одновременной работы
		const numOps = 10
		done := make(chan bool, numOps*2)

		// запускаем горутины для записи
		for i := 0; i < numOps; i++ {
			clientID := fmt.Sprintf("client%d", i)
			state := &BucketState{
				Tokens:     float64(i),
				Capacity:   float64(i * 10),
				Rate:       float64(i),
				LastUpdate: time.Now().UTC(),
			}

			go func() {
				err := fs.Save(clientID, state)
				assert.NoError(t, err)
				done <- true
			}()
		}

		// запускаем горутины для чтения
		for i := 0; i < numOps; i++ {
			go func() {
				_, err = fs.LoadAll()
				// не проверяем результат
				done <- true
			}()
		}

		for i := 0; i < numOps*2; i++ {
			<-done
		}

		// проверяем
		clients, err := fs.LoadAll()
		require.NoError(t, err)
		assert.Len(t, clients, numOps)
	})

	t.Run("Delete", func(t *testing.T) {
		tempFile := createTempFile(t)

		initialData := map[string]*BucketState{
			"client1": {
				Tokens:     5,
				Capacity:   10,
				Rate:       1,
				LastUpdate: time.Now().UTC(),
			},
			"client2": {
				Tokens:     8,
				Capacity:   10,
				Rate:       2,
				LastUpdate: time.Now().UTC(),
			},
		}
		data, err := json.MarshalIndent(initialData, "", "  ")
		require.NoError(t, err)
		err = os.WriteFile(tempFile, data, 0644)
		require.NoError(t, err)

		fs, err := NewFileStorage(tempFile)
		require.NoError(t, err)

		err = fs.Delete("client1")
		require.NoError(t, err)

		clients, err := fs.LoadAll()
		require.NoError(t, err)
		assert.NotContains(t, clients, "client1")
		assert.Contains(t, clients, "client2")
	})

	t.Run("LoadAll with empty file", func(t *testing.T) {
		tempFile := createTempFile(t)
		fs, err := NewFileStorage(tempFile)
		require.NoError(t, err)

		clients, err := fs.LoadAll()
		require.Error(t, err)
		assert.Empty(t, clients)
	})
}
