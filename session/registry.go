package session

import (
	"strings"
	"sync"
)

type Registry struct {
	sessions map[string]*Session
	mu       sync.RWMutex
}

func NewRegistry() *Registry {
	return &Registry{
		sessions: make(map[string]*Session),
	}
}

func (r *Registry) AddSession(xuid string, session *Session) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions[xuid] = session
}

func (r *Registry) GetSession(xuid string) *Session {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.sessions[xuid]
}

func (r *Registry) GetSessionByUsername(username string) *Session {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, session := range r.sessions {
		if strings.EqualFold(session.client.IdentityData().DisplayName, username) {
			return session
		}
	}
	return nil
}

func (r *Registry) RemoveSession(xuid string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sessions, xuid)
}

func (r *Registry) GetSessions() []*Session {
	r.mu.RLock()
	defer r.mu.RUnlock()

	sessions := make([]*Session, 0)
	for _, session := range r.sessions {
		sessions = append(sessions, session)
	}
	return sessions
}
