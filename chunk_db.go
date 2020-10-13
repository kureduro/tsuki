package tsuki

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"sync"
	"syscall"
)


const (
    ErrChunkExists = ChunkError("chunk already exists")
    ErrChunkNotFound = ChunkError("chunk does not exists")
)

type ChunkError string

func (c ChunkError) Error() string { return string(c) }



type ChunkDB interface {
    Get(id string) (io.Reader, func(), error)
    Create(id string) (io.Writer, func(), error)
    Exists(id string) bool

    // Should be concurrency safe
    Remove(id string) error
    
    BytesAvailable() int
}



type InMemoryChunkStorage struct {
    Index map[string]string
    Mu sync.RWMutex
    accessCount sync.WaitGroup
    callsPerformed int
}

func NewInMemoryChunkStorage(index map[string]string) *InMemoryChunkStorage {
    return &InMemoryChunkStorage {
        Index: index,
    }
}

func (s *InMemoryChunkStorage) Get(id string) (io.Reader, func(), error) {
    if !s.Exists(id) {
        return nil, func(){}, ErrChunkNotFound
    }

    s.accessCount.Add(1)

    s.Mu.RLock()
    defer s.Mu.RUnlock()

    chunk := s.Index[id]

    buf := bytes.NewBufferString(chunk)

    closeFunc := func() {
        s.accessCount.Done()
    }

    return buf, closeFunc, nil
}

func (s *InMemoryChunkStorage) Create(id string) (io.Writer, func(), error) {
    if s.Exists(id) {
        return nil, func(){}, ErrChunkExists
    }

    s.accessCount.Add(1)

    s.Mu.Lock()
    defer s.Mu.Unlock()

    s.Index[id] = ""

    buf := &bytes.Buffer{}

    writeChunk := func() {
        str := &strings.Builder{}
        io.Copy(str, buf)
        s.Index[id] = str.String()

        s.accessCount.Done()
    }

    return buf, writeChunk, nil
}

func (s *InMemoryChunkStorage) Exists(id string) (exists bool) {
    s.accessCount.Add(1)
    defer s.accessCount.Done()

    s.Mu.RLock()
    defer s.Mu.RUnlock()

    _, exists = s.Index[id]
    return
}

func (s *InMemoryChunkStorage) Remove(id string) error {
    s.accessCount.Add(1)
    defer s.accessCount.Done()

    s.Mu.Lock()
    defer s.Mu.Unlock()

    delete(s.Index, id)
    return nil
}

func (s *InMemoryChunkStorage) BytesAvailable() int {
    return 1024 * 1024 * 10
}

/*
    Get(id string) (io.Reader, func(), error)
    Create(id string) (io.Writer, func(), error)
    Exists(id string) bool

    // Should be concurrency safe
    Remove(id string) error
    
    BytesAvailable() int
    
     ------

    Index map[string]string
    Mu sync.RWMutex
    accessCount sync.WaitGroup
    callsPerformed int
*/

type FileSystemChunkStorage struct {
    Dir string
    index map[string]*sync.RWMutex
    mu sync.RWMutex
}

func NewFileSystemChunkStorage(dir string) (*FileSystemChunkStorage, error) {
    err := os.RemoveAll(dir)
    if err != nil {
        return nil, fmt.Errorf("clear storage: %v", err)
    }

    err = os.Mkdir(dir, 0666)
    if err != nil {
        return nil, fmt.Errorf("clear storage: %v", err)
    }

    store := &FileSystemChunkStorage{
        Dir: dir,
        index: make(map[string]*sync.RWMutex),
    }

    return store, nil
}

func (s *FileSystemChunkStorage) Create(id string) (io.Writer, func(), error) {
    if s.Exists(id) {
        return nil, func(){}, fmt.Errorf("create chunk: %s already exists", id)
    }

    file, err := os.Create(path.Join(s.Dir, id))
    if err != nil {
        return nil, func(){}, fmt.Errorf("create chunk: %v", err)
    }

    mu := &sync.RWMutex{}
    mu.Lock()  // begin

    closeChunk := func() {
        file.Close()

        mu.Unlock()  // end
    }

    s.mu.Lock()
    defer s.mu.Unlock()

    s.index[id] = mu

    return file, closeChunk, nil
}

func (s *FileSystemChunkStorage) Get(id string) (io.Reader, func(), error) {
    file, err := os.Open(id)
    if err != nil {
        return nil, func(){}, fmt.Errorf("get chunk: %v", err)
    }

    s.mu.RLock()
    mu, exists := s.index[id]
    s.mu.RUnlock()

    if !exists {
        file.Close()

        return nil, func(){}, fmt.Errorf("get chunk: %s not found", id)
    }

    mu.RLock() // begin

    closeChunk := func() {
        file.Close()

        mu.RUnlock()  // end
    }

    return file, closeChunk, nil
}

func (s *FileSystemChunkStorage) Exists(id string) bool {
    s.mu.RLock()
    defer s.mu.RUnlock()

    _, exists := s.index[id]
    return exists
}

func (s *FileSystemChunkStorage) Remove(id string) error {
    s.mu.RLock()
    mu, exists := s.index[id]
    s.mu.RUnlock()

    if !exists {
        return fmt.Errorf("remove chunk: %s does not exist", id)
    }

    mu.Lock()
    defer mu.Unlock()

    err := os.Remove(path.Join(s.Dir, id))
    if err != nil {
        return fmt.Errorf("remove chunk: %v", err)
    }

    return nil
}

func (s *FileSystemChunkStorage) BytesAvailable() int {
    var stat syscall.Statfs_t
    syscall.Statfs(s.Dir, &stat)

    return int(stat.Bavail * uint64(stat.Bsize))
}


