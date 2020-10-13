package tsuki

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
)

type ChunkError string

func (c ChunkError) Error() string { return string(c) }

const (
    ErrChunkExists = ChunkError("chunk already exists")
    ErrChunkNotFound = ChunkError("chunk does not exists")
)

type ExpectAction int

const (
    ExpectActionNothing = ExpectAction(0)
    ExpectActionRead = ExpectAction(1)
    ExpectActionWrite = ExpectAction(2)
)

var strToExpectAction = map[string]ExpectAction {
    "read": ExpectActionRead,
    "write": ExpectActionWrite,
}

// token -> chunkId -> ExpectAction
// Putting token first makes it more cache-friendly, since there much less
// tokens than chunks.
//type TokenExpectations map[string]map[string]ExpectAction
type TokenExpectation struct {
    action ExpectAction
    processedChunks map[string]bool
    pendingCount int
    mu sync.RWMutex
}

type ExpectationDB struct {
    mu sync.RWMutex
    index map[string]*TokenExpectation

    expectsPerChunk map[string]int
    purgeChunk map[string]struct{}
}

func NewExpectationDB() *ExpectationDB {
    return &ExpectationDB {
        index: make(map[string]*TokenExpectation),
        expectsPerChunk: make(map[string]int),
        purgeChunk: make(map[string]struct{}),
    }
}

func (e *ExpectationDB) Get(token string) *TokenExpectation {
    e.mu.RLock()
    defer e.mu.RUnlock()

    return e.index[token]
}

func (e *ExpectationDB) Set(token string, exp *TokenExpectation) {
    e.mu.Lock()
    defer e.mu.Unlock()

    e.index[token] = exp

    for k := range exp.processedChunks {
        e.expectsPerChunk[k]++
    }
}

func (e *ExpectationDB) Remove(token string) []string {
    e.mu.Lock()
    defer e.mu.Unlock()

    toPurge := make([]string, 0, len(e.purgeChunk))
    for id := range e.index[token].processedChunks {
        e.expectsPerChunk[id]--

        if e.expectsPerChunk[id] == 0 {
            delete(e.expectsPerChunk, id)

            _, obsolete := e.purgeChunk[id]
            if obsolete {
                toPurge = append(toPurge, id)
                delete(e.purgeChunk, id)
            }
        }
    }

    delete(e.index, token)

    return toPurge
}

func (e *ExpectationDB) MakeObsolete(chunks ...string) (toPurge []string) {
    e.mu.Lock()
    defer e.mu.Unlock()

    for _, id := range chunks {
        expCount := e.expectsPerChunk[id]
        if expCount == 0 {
            // Note that there's no `id` in the e.purgeChunk if we've entered
            // this if. The only way to make chunk obsolete is to call this 
            // function. Hence, if MakeObsolete didn't remove it the first 
            // call, there was at least one expect action associated with 
            // this chunk. And because of this, the Remove will return it
            // and clear e.purgeChunk.

            toPurge = append(toPurge, id)
            continue
        }

        e.purgeChunk[id] = struct{}{}
    }

    return
}

type FSProbeInfo struct {
    Available int
}

type ChunkDB interface {
    Get(id string) (io.Reader, func(), error)
    Create(id string) (io.Writer, func(), error)
    Exists(id string) bool

    // Should be concurrency safe
    Remove(id string) error
    
    BytesAvailable() int
}

type FileServer struct {
    chunks ChunkDB
    expectations *ExpectationDB
    nsConn NSConnector

    // clientHandler ...also, maybe
    innerHandler http.Handler
}

func NewFileServer(store ChunkDB, nsConn NSConnector) (s *FileServer) {
    s = &FileServer{
        chunks: store,
        expectations: NewExpectationDB(),
        nsConn: nsConn,
    }


    innerRouter := http.NewServeMux()
    innerRouter.Handle("/expect/", http.HandlerFunc(s.ExpectHandler))
    innerRouter.Handle("/cancelToken", http.HandlerFunc(s.CancelTokenHandler))
    innerRouter.Handle("/purge", http.HandlerFunc(s.PurgeHandler))
    innerRouter.Handle("/probe", http.HandlerFunc(s.ProbeHandler))
    innerRouter.Handle("/replicate", http.HandlerFunc(s.ReplicateHandler))

    s.innerHandler = innerRouter

    return
}

func (s *FileServer) Expect(token string, action ExpectAction, chunks ...string) error {
    // TODO: timeout
    exp := s.expectations.Get(token)
    if exp != nil {
        return fmt.Errorf("expect group already exists, token=%s", token)
    }

    exp = &TokenExpectation {
        action: action,
        processedChunks: make(map[string]bool),
        pendingCount: len(chunks),
    }

    for _, id := range chunks {
        // This if looks kinda crammed and out of context....
        // What if we have more types of expect actions?
        if action == ExpectActionRead && !s.chunks.Exists(id) {
            return ErrChunkNotFound
        }
        exp.processedChunks[id] = false
    }

    s.expectations.Set(token, exp)

    return nil
}

func (s *FileServer) fulfillExpectation(token, id string) {
    // Expects correct token and id

    exp := s.expectations.Get(token)

    exp.mu.Lock()
    defer exp.mu.Unlock()

    _, chunkExists := exp.processedChunks[id]
    if !chunkExists {
        log.Printf("warning: attempt to fulfill expectation for wrong chunk. token=%s, chunk=%s", token, id)
        return
    }

    exp.processedChunks[id] = true
    exp.pendingCount--

    if exp.pendingCount == 0 {
        toPurge := s.expectations.Remove(token)

        for _, id := range toPurge {
            go s.chunks.Remove(id)
        }
    }
}

func (s *FileServer) GetTokenExpectationForChunk(token, id string) ExpectAction {
    e := s.expectations.Get(token)
    if e == nil {
        return ExpectActionNothing
    }

    e.mu.RLock()
    defer e.mu.RUnlock()

    processed, authorized := e.processedChunks[id]
    if !authorized || processed {
        return ExpectActionNothing
    }

    return e.action
}

func (s *FileServer) ServeNS(w http.ResponseWriter, r *http.Request) {
    log.Printf("ServeInner: %s", r.URL)

    // If NS hasn't probed server, anybody can access NS API
    // The address of NS should be stored on disk and loaded on startup
    // to prevent this.
    if !s.nsConn.IsNS(r.RemoteAddr) {
        w.WriteHeader(http.StatusUnauthorized)
        return
    }

    // TODO: Block non NS
    s.innerHandler.ServeHTTP(w, r)
}

func (s *FileServer) ExpectHandler(w http.ResponseWriter, r *http.Request) {
    actionStr := r.URL.Query().Get("action")
    action, correct := strToExpectAction[actionStr]

    if !correct {
        w.WriteHeader(http.StatusBadRequest)
        fmt.Fprint(w, "Not correct action")
        return
    }

    token := strings.TrimPrefix(r.URL.Path, "/expect/")

    // TODO: This check may be unneeded when trailing slash is omitted when
    // nothing follows it.
    if token == "" {
        w.WriteHeader(http.StatusBadRequest)
        fmt.Fprint(w, "Token is empty")
        return
    }

    buf := &bytes.Buffer{}
    io.Copy(buf, r.Body)

    var chunks []string
    if err := json.Unmarshal(buf.Bytes(), &chunks); err != nil {
        w.WriteHeader(http.StatusBadRequest)
        fmt.Fprint(w, err)
        return
    }

    mock := r.Header.Get("mock")
    if mock == "mock" {
        for _, id := range chunks {
            s.nsConn.ReceivedChunk(id)
        }
    }

    err := s.Expect(token, action, chunks...)
    if err != nil {
        w.WriteHeader(http.StatusForbidden)
        fmt.Fprint(w, err)
        return
    }

    log.Printf("%s : %v", r.URL, chunks)

    w.WriteHeader(http.StatusOK)
}

func (s *FileServer) CancelTokenHandler(w http.ResponseWriter, r *http.Request) {
    token := r.URL.Query().Get("token")

    if token == "" {
        w.WriteHeader(http.StatusBadRequest)
        return
    }

    exp := s.expectations.Get(token)
    if exp == nil {
        w.WriteHeader(http.StatusOK)
        return
    }

    exp.mu.Lock()
    defer exp.mu.Unlock()


    toUndo := make([]string, 0, len(exp.processedChunks) - exp.pendingCount)
    for k, v := range exp.processedChunks {
        if v {
            toUndo = append(toUndo, k)
        }
    }

    var toPurge []string
    if exp.action == ExpectActionWrite && len(toUndo) != 0 {
        toPurge = s.expectations.MakeObsolete(toUndo...)
    }

    exp.action = ExpectActionNothing

    toPurge = append(toPurge, s.expectations.Remove(token)...)
    for _, id := range toPurge {
        go s.chunks.Remove(id)
    }

    w.WriteHeader(http.StatusOK)
}

func (s *FileServer) PurgeHandler(w http.ResponseWriter, r *http.Request) {
    buf := &bytes.Buffer{}
    io.Copy(buf, r.Body)

    var chunks []string
    if err := json.Unmarshal(buf.Bytes(), &chunks); err != nil {
        w.WriteHeader(http.StatusBadRequest)
        return
    }

    toPurge := s.expectations.MakeObsolete(chunks...)
    for _, id := range toPurge {
        go s.chunks.Remove(id)
    }

    w.WriteHeader(http.StatusOK)
}

func (s *FileServer) ProbeHandler(w http.ResponseWriter, r *http.Request) {
    if !s.nsConn.IsNS(r.RemoteAddr) {
        w.WriteHeader(http.StatusUnauthorized)
        return
    }

    log.Print("Probed")

    // TODO: inject this functionality and test it.
    save, err := os.Create(".tsukifs")
    if err == nil {
        fmt.Fprint(save, r.RemoteAddr)
        save.Close()
    }

    s.nsConn.SetNSAddr(r.RemoteAddr)

    info := s.GenerateProbeInfo()

    probeBytes, err := json.Marshal(info)
    if err != nil {
        w.WriteHeader(http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusOK)
    fmt.Fprint(w, string(probeBytes))
}

func (s *FileServer) ReplicateHandler(w http.ResponseWriter, r *http.Request) {
    token := r.URL.Query().Get("token")
    destIP := r.URL.Query().Get("addr")

    // TODO: remove copypasta
    buf := &bytes.Buffer{}
    io.Copy(buf, r.Body)

    var chunks []string
    if err := json.Unmarshal(buf.Bytes(), &chunks); err != nil {
        w.WriteHeader(http.StatusBadRequest)
        return
    }

    for _, id := range chunks {
        err := s.Expect(token, ExpectActionRead, id)
        if err != nil {
            log.Printf("error: replica could not be registered internally, token=%s, chunkId=%s", token, id)
            continue
        }

        defer s.fulfillExpectation(token, id)

        chunk, closeChunk, err := s.chunks.Get(id)

        if err != nil {
            w.WriteHeader(http.StatusBadRequest)
            return
        }
        defer closeChunk()


        destAddr := fmt.Sprintf("http://%s/chunks/%s?token=%s", destIP, id, token)
        resp, err := http.Post(destAddr, "application/octet-stream", chunk)

        if err != nil {
            log.Printf("warning: could not replicate chunk to %s, %v.", destAddr, err)
            continue
        }
        defer resp.Body.Close()

        status := resp.StatusCode
        if status != http.StatusOK {
            log.Printf("warning: chunk replica was not accepted by %s, response status code: %d", 
                        destAddr, status)
        }
    }

    w.WriteHeader(http.StatusOK)
}

func (s *FileServer) GenerateProbeInfo() *FSProbeInfo {
    return &FSProbeInfo {
        Available: s.chunks.BytesAvailable(),
    }
}

func (cs *FileServer) ServeClient(w http.ResponseWriter, r *http.Request) {
    chunkId := strings.TrimPrefix(r.URL.Path, "/chunks/")
    token := r.URL.Query().Get("token")
    
    switch r.Method {
    case http.MethodGet:
        cs.SendChunk(w, r, chunkId, token)
    case http.MethodPost:
        cs.ReceiveChunk(w, r, chunkId, token)
    default:
        w.WriteHeader(http.StatusMethodNotAllowed)
    }
}

func (s *FileServer) SendChunk(w http.ResponseWriter, r *http.Request, id, token string) {
    log.Printf("Chunk READ request: id=%s, token=%s", id, token)

    if s.GetTokenExpectationForChunk(token, id) == ExpectActionNothing {
        w.WriteHeader(http.StatusUnauthorized)
        return
    }
    defer s.fulfillExpectation(token, id)

    chunk, closeChunk, err := s.chunks.Get(id)
    defer closeChunk()

    if err != nil {
        w.WriteHeader(http.StatusNotFound)
        return
    }

    io.Copy(w, chunk)
    return
}

func (s *FileServer) ReceiveChunk(w http.ResponseWriter, r *http.Request, id, token string) {
    log.Printf("Chunk WRITE request: id=%s, token=%s", id, token)

    if s.GetTokenExpectationForChunk(token, id) == ExpectActionNothing {
        w.WriteHeader(http.StatusUnauthorized)
        return
    }
    defer s.fulfillExpectation(token, id)

    chunk, finishChunk, err := s.chunks.Create(id)
    defer finishChunk()

    if err == ErrChunkExists {
        w.WriteHeader(http.StatusForbidden)
        return
    }

    if err != nil {
        w.WriteHeader(http.StatusInternalServerError)
        return
    }

    io.Copy(chunk, r.Body)

    s.nsConn.ReceivedChunk(id)
    w.WriteHeader(http.StatusOK)
}
