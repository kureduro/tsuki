package main

import (
	"net/http"
	"sync"
	"time"
)

type FSStatus int

const (
	LIVE           FSStatus = 2
	PARTIALLY_DEAD FSStatus = 1
	DEAD           FSStatus = 0
)

type FileServerInfo struct {
	mu        sync.Mutex
	Host      string
	Port      int
	Alive     bool
	Status    FSStatus
	NextAlive int
	LastPulse time.Time
	ID        int
}

type PoolInfo struct {
	mu        sync.Mutex
	StorageNodes   []*FileServerInfo
	SoftPulseQueue chan int
	HardPulseQueue chan int
	http.Handler
	Next  int
	Alive int
}

func InitFServers(conf *Config) *PoolInfo {
	storage := PoolInfo{
		SoftPulseQueue: make(chan int, 1),
		HardPulseQueue: make(chan int, 1),
	}
	for i, storageNode := range conf.Storage {
		storage.StorageNodes = append(storage.StorageNodes,
			&FileServerInfo{
				Host:      storageNode.Host,
				Port:      storageNode.Port,
				Alive:     false,
				NextAlive: (i + 1) % len(conf.Storage),
				ID:        i,
			})
	}

	return &storage
}

func (s *PoolInfo) Select() *FileServerInfo {
	next := s.StorageNodes[s.Next]

	if !next.Alive {
		next = s.StorageNodes[next.NextAlive]
	}

	s.Next = next.NextAlive

	return next
}

func (s *PoolInfo) SelectSeveralExcept(except []string, num int) []*FileServerInfo {
	exceptMap := map[string]int{}
	for _, host := range except {
		exceptMap[host] = 1
	}
	if s.Alive-len(except) < num {
		num = s.Alive - len(except)
	}

	selected := []*FileServerInfo{}

	next := s.StorageNodes[s.Next]
	for i := 0; i < num; {
		if !next.Alive || exceptMap[next.Host] == 0 {
		} else {
			selected = append(selected, next)
			i++
		}
		next = s.StorageNodes[next.NextAlive]
	}

	return selected
}

func (s *PoolInfo) setNewAliveDead(nowDeadID int) {
	s.setNewAlive(s.StorageNodes[nowDeadID].NextAlive, nowDeadID)
}

func (s *PoolInfo) setNewAlive(newAliveID int, cur int) {
	setOne := false

	s.mu.Lock()
	for i := cur; i >= 0; i-- {
		node := s.StorageNodes[i]
		node.NextAlive = newAliveID
		if node.Alive {
			setOne = true
			break
		}
	}
	s.mu.Unlock()

	if !setOne {
		if cur != len(s.StorageNodes) - 1 {
			s.setNewAlive(newAliveID, len(s.StorageNodes)-1)
		} else {
			// no alive node; die
			// panic("no alive node")
		}
	}
}


func (s *PoolInfo) ChangeStatus(id int, status FSStatus) {
	node := s.StorageNodes[id]
	node.Status = status

	node.mu.Lock()
	node.Alive = status == LIVE
	node.mu.Unlock()

	if node.Alive {
		num := len(s.StorageNodes)
		s.setNewAlive(id, ((id - 1) % num + num) % num)
	} else {
		s.setNewAliveDead(id)
	}
}

func (s *PoolInfo) IsDead(id int, soft bool) bool {
	return soft && s.StorageNodes[id].Status == PARTIALLY_DEAD || !soft && s.StorageNodes[id].Status == DEAD
}