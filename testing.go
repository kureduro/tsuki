package tsuki

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"
)

func NewGetChunkRequest(id, token string) *http.Request {
    req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("/chunks/%s?token=%s", id, token), nil)
    return req
}

func NewPostChunkRequest(id, content, token string) *http.Request {
    buf := bytes.NewBufferString(content)
    request, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("/chunks/%s?token=%s", id, token), buf)
    return request
}

func NewExpectRequest(action, token string, chunks ...string) *http.Request {
    b, _ := json.Marshal(chunks)
    url := fmt.Sprintf("/expect/%s?action=%s", token, action)
    req, _ := http.NewRequest(http.MethodGet, url, bytes.NewBuffer(b))
    return req
}

func NewCancelTokenRequest(token string) *http.Request {
    url := fmt.Sprintf("/cancelToken?token=%s", token)
    req, _ := http.NewRequest(http.MethodPost, url, nil)
    return req
}

func NewPurgeRequest(chunks ...string) *http.Request {
    b, _ := json.Marshal(chunks)
    url := "/purge"
    req, _ := http.NewRequest(http.MethodGet, url, bytes.NewBuffer(b))
    return req
}

func NewProbeRequest(remoteAddr string) *http.Request {
    req, _ := http.NewRequest(http.MethodGet, "/probe", nil)
    req.RemoteAddr = remoteAddr
    return req
}

func NewReplicateRequest(addr, token string, chunks ...string) *http.Request {
    b, _ := json.Marshal(chunks)
    url := fmt.Sprintf("/replicate?addr=%s&token=%s", addr, token)
    req, _ := http.NewRequest(http.MethodGet, url, bytes.NewBuffer(b))
    return req
}

func AssertChunkContents(t *testing.T, chunks ChunkDB, id, want string) {
    t.Helper()

    if !chunks.Exists(id) {
        t.Fatalf("expected chunk %v to exist, but it doesn't", id)
    }

    chunk, closeChunk, err := chunks.Get(id)
    if err != nil {
        t.Fatalf("chunk was deleted right after it was checked for existance")
    }
    defer closeChunk()

    got := &strings.Builder{}
    _, _ = io.Copy(got, chunk)


    if got.String() != want {
        t.Errorf("got chunk contents %q, want %q", got, want)
    }
}

func AssertChunkDoesntExists(t *testing.T, chunks ChunkDB, id string) {
    t.Helper()

    if chunks.Exists(id) {
        t.Errorf("chunk %v exists, but it shouldn't", id)
    }
}

func AssertStatus(t *testing.T, got, want int) {
    t.Helper()
    if got != want {
        t.Errorf("incorrect response code, got %d, want %d", got, want)
    }
}

func AssertResponseBody(t *testing.T, got, want string) {
    t.Helper()
    if got != want {
        t.Errorf("wrong response body, got %q, want %q", got, want)
    }
}

func AssertReceivedChunkCalls(t *testing.T, nsConn *SpyNSConnector, ids ...string) {
    t.Helper()

    if !reflect.DeepEqual(nsConn.receivedChunks, ids) {
        t.Errorf("incorrect calls to ns/receivedChunk, got %#v, %#v", nsConn.receivedChunks, ids)
    }
}


type SpyPoller struct {
    CallCount int
}

func (p *SpyPoller) Poll() {
    p.CallCount++
}


type SpySleeper struct {
    CallCount int
}

func (s *SpySleeper) Sleep() {
    s.CallCount++
}


type SpySleeperTime struct {
    DurationSlept time.Duration
}

func (s *SpySleeperTime) Sleep(duration time.Duration) {
    s.DurationSlept = duration
}

