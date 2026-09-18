package local

import (
	"context"
	"sync"
)

type localAdmissionPermit struct {
	gate   *LocalAdmissionGate
	scopes []string
	once   sync.Once
}

func (p *localAdmissionPermit) Release(context.Context) error {
	p.once.Do(func() {
		p.gate.mu.Lock()
		defer p.gate.mu.Unlock()
		p.gate.active--
		for _, scope := range p.scopes {
			p.gate.states[scope].active--
		}
		p.gate.signal()
	})
	return nil
}
