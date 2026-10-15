package maintenance

type Mode struct {
	Mode string `json:"mode"`
}

type CacheStatus struct {
	Mode  string `json:"mode"`
	Bytes int64  `json:"bytes"`
	Limit int64  `json:"limit"`
}

type BackupStatus struct {
	State     string `json:"state,omitempty"`
	Enabled   bool   `json:"-"`
	Encrypted bool   `json:"encrypted"`
	LastError string `json:"-"`
}

type AnalysisStatus struct {
	Mode  string `json:"mode"`
	State string `json:"state"`
}

type Status struct {
	Mode      string         `json:"mode"`
	Activity  string         `json:"activity"`
	Attention bool           `json:"attention"`
	Library   Mode           `json:"library"`
	Metadata  Mode           `json:"metadata"`
	Analysis  AnalysisStatus `json:"analysis"`
	Cache     CacheStatus    `json:"cache"`
	Backups   BackupStatus   `json:"backups"`
}

func (manager *Manager) Status() Status {
	cacheBytes, cacheErr := manager.dependencies.CacheStats()
	backup := manager.dependencies.BackupStatus()
	backup.State = "automatic"
	if !backup.Enabled {
		backup.State = "needs-setup"
	} else if backup.LastError != "" {
		backup.State = "error"
	}
	metadataMode := "local"
	if manager.dependencies.MetadataConfigured() {
		metadataMode = "automatic"
	}
	activity := "idle"
	if manager.Busy() {
		activity = "busy"
	}
	return Status{
		Mode:      "automatic",
		Activity:  activity,
		Attention: backup.State != "automatic" || cacheErr != nil,
		Library:   Mode{"automatic"},
		Metadata:  Mode{metadataMode},
		Analysis:  AnalysisStatus{"on-change", manager.dependencies.AnalysisStatus()},
		Cache:     CacheStatus{"bounded", cacheBytes, manager.limit},
		Backups:   backup,
	}
}
