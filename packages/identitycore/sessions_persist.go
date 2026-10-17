package identitycore

type sessionChange struct {
	set, before, removed map[string]Session
	existed              map[string]bool
}

func applySessionChange(values, set map[string]Session, remove []string) sessionChange {
	change := sessionChange{set: set, before: make(map[string]Session, len(set)), removed: make(map[string]Session, len(remove)), existed: make(map[string]bool, len(set))}
	for key, session := range set {
		change.before[key], change.existed[key] = values[key]
		values[key] = session
	}
	for _, key := range remove {
		if session, found := values[key]; found {
			change.removed[key] = session
			delete(values, key)
		}
	}
	return change
}

func (change sessionChange) empty() bool {
	return len(change.set) == 0 && len(change.removed) == 0
}

func (change sessionChange) undo(values map[string]Session) {
	for key, session := range change.set {
		if current, found := values[key]; !found || current != session {
			continue
		}
		if change.existed[key] {
			values[key] = change.before[key]
		} else {
			delete(values, key)
		}
	}
	for key, session := range change.removed {
		if _, found := values[key]; !found {
			values[key] = session
		}
	}
}

func (sessions *Sessions) persistChange(change sessionChange) error {
	sessions.config.Mutex.RLock()
	err := sessions.config.Persist(sessions.config.File, *sessions.config.Values)
	sessions.config.Mutex.RUnlock()
	if err != nil {
		sessions.config.Mutex.Lock()
		change.undo(sessions.values())
		sessions.config.Mutex.Unlock()
	}
	return err
}
