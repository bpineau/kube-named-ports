// Package worker ensure the expected named ports are set on all node pools.
package worker

import (
	"sync"
	"time"

	"github.com/bpineau/kube-named-ports/config"
	np "github.com/bpineau/kube-named-ports/pkg/namedports"
)

// Worker ensure the expected named ports are set on all node pools.
type Worker interface {
	Start()
	Stop()
	Add(name string, port int64)
}

// PortMapper is worker synchronizing GCP named ports and services annotations
type PortMapper struct {
	expectedLock sync.RWMutex
	expected     np.PortList
	stop         chan bool
	config       *config.KnpConfig
}

var syncDelay = 60 * time.Second

// NewWorker returns a PortMapper worker
func NewWorker(config *config.KnpConfig) *PortMapper {
	p := &PortMapper{
		expected: make(np.PortList),
		stop:     make(chan bool),
		config:   config,
	}
	return p
}

// Start launchs the PortMapper worker
func (p *PortMapper) Start() {
	go p.syncNamedPorts()
}

// Stop stops the PortMapper worker
func (p *PortMapper) Stop() {
	p.stop <- true
}

// Add declares a named port we want to keep in sync with GCP
func (p *PortMapper) Add(name string, port int64) {
	p.expectedLock.Lock()
	defer p.expectedLock.Unlock()
	p.expected[name] = port
}

func (p *PortMapper) syncNamedPorts() {
	namer := np.NewNamedPort(
		p.config.Zone,
		p.config.Cluster,
		p.config.Project,
		p.config.DryRun,
		p.config.Logger)
	portscopy := np.PortList{}

	for {
		select {
		case <-time.After(syncDelay):
			p.expectedLock.RLock()
			for k, v := range p.expected {
				portscopy[k] = v
			}
			p.expectedLock.RUnlock()

			err := namer.ResyncNamedPorts(portscopy)
			if err != nil {
				p.config.Logger.Errorf("Error during ports resync: %v", err)
			}
		case <-p.stop:
			return
		}
	}
}
