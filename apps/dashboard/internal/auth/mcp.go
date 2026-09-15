package auth

import (
	"context"
	"errors"
)

const mcpDevice = "MCP client"

// IssueMCPToken replaces the one full-owner MCP token and returns it once.
func (manager *Manager) IssueMCPToken(ctx context.Context) (Session, error) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if manager.owner.ID == "" {
		return Session{}, ErrNotConfigured
	}
	previous := append([]sessionState(nil), manager.sessions...)
	manager.removeMCPState()
	session, state := manager.newSession(mcpDevice, "mcp")
	manager.sessions = append([]sessionState{state}, manager.sessions...)
	if len(manager.sessions) > maxSessions {
		manager.sessions = manager.sessions[:maxSessions]
	}
	if err := manager.saveSessions(ctx); err != nil {
		manager.sessions = previous
		return Session{}, ErrState
	}
	session.ID, session.Name = manager.owner.ID, manager.owner.Name
	return session, nil
}

// RevokeMCPToken removes every token created for MCP access.
func (manager *Manager) RevokeMCPToken(ctx context.Context) error {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	previous := append([]sessionState(nil), manager.sessions...)
	if !manager.removeMCPState() {
		return nil
	}
	if err := manager.saveSessions(ctx); err != nil {
		manager.sessions = previous
		return errors.New("could not revoke MCP access")
	}
	return nil
}

func (manager *Manager) removeMCPState() bool {
	kept := manager.sessions[:0]
	for _, session := range manager.sessions {
		if session.Purpose != "mcp" {
			kept = append(kept, session)
		}
	}
	changed := len(kept) != len(manager.sessions)
	manager.sessions = kept
	return changed
}
