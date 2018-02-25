// Package worker ensure the expected named ports are set on all node pools.
package worker

import (
	"fmt"
	"sync"
)

// Worker ensure the expected named ports are set on all node pools.
type Worker interface {
	Start()
	Add(name string, port int) error
}

// PortMapper is worker synchronizing GCP named ports and services annotations
type PortMapper struct {
	//existingLock sync.RWMutex
	existing     map[string]int
	expectedLock sync.RWMutex
	expected     map[string]int
}

// NewWorker returns a PortMapper worker
func NewWorker() *PortMapper {
	p := &PortMapper{
		existing: make(map[string]int),
		expected: make(map[string]int),
	}
	return p
}

// Start launchs the PortMapper worker
func (p *PortMapper) Start() {
	// go watchExistingNamedPorts(); go applyAllNamedPorts(); // select {} ;
	go p.syncNamedPorts()
}

// Add declares a named port we want to keep in sync with GCP
func (p *PortMapper) Add(name string, port int) error {
	p.expectedLock.Lock()
	defer p.expectedLock.Unlock()

	p.expected[name] = port

	fmt.Printf("name=%s port=%d\n", name, port)
	return nil
}

func (p *PortMapper) syncNamedPorts() {
	select {}
}
