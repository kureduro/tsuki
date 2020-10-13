package tsuki

import (
    "sync"
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

