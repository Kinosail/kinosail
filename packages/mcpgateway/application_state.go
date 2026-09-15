package mcpgateway

import "path/filepath"

type applicationState struct {
	dataDir string
	load    func(string, any) (bool, error)
	save    func(string, any) error
}

// ApplicationState binds connection state to the installation's persistence operations.
func ApplicationState(dataDir string, load func(string, any) (bool, error), save func(string, any) error) StateStore {
	return applicationState{dataDir: dataDir, load: load, save: save}
}

func (store applicationState) Load(target any) (bool, error) {
	if store.dataDir == "" {
		return false, nil
	}
	return store.load(filepath.Join(store.dataDir, StateFilename), target)
}

func (store applicationState) Save(value any) error {
	return store.save(filepath.Join(store.dataDir, StateFilename), value)
}
