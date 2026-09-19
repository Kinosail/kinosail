package transcodepolicy

import (
	"context"
	"sync"
)

// CheckCoordinatorDependencies binds one application's persisted check state
// to the shared transcoder policy workflow.
type CheckCoordinatorDependencies struct {
	Lock           *sync.RWMutex
	Result         *CheckResult
	FFmpeg         string
	Resolve        func() (Settings, error)
	Pending        func() Settings
	BackendName    func(string) string
	RecordHardware func(Settings, CheckResult)
}

// CheckCoordinator owns explicit transcoder checks and their stable result.
type CheckCoordinator struct {
	dependencies CheckCoordinatorDependencies
}

// NewCheckCoordinator binds app state to the shared check lifecycle.
func NewCheckCoordinator(dependencies CheckCoordinatorDependencies) CheckCoordinator {
	return CheckCoordinator{dependencies: dependencies}
}

// Run executes one check, updates hardware verification, and stores its result.
func (coordinator CheckCoordinator) Run(ctx context.Context) CheckResult {
	options, _ := coordinator.dependencies.Resolve()
	result := Check(ctx, coordinator.dependencies.FFmpeg, options, coordinator.dependencies.BackendName(options.Accelerator))
	coordinator.dependencies.RecordHardware(options, result)
	coordinator.dependencies.Lock.Lock()
	*coordinator.dependencies.Result = result
	coordinator.dependencies.Lock.Unlock()
	return result
}

// Current returns the stored result or a stable pending view before the first check.
func (coordinator CheckCoordinator) Current() CheckResult {
	coordinator.dependencies.Lock.RLock()
	result := *coordinator.dependencies.Result
	coordinator.dependencies.Lock.RUnlock()
	if result.Status == "" {
		options := coordinator.dependencies.Pending()
		return PendingCheck(options, coordinator.dependencies.BackendName(options.Accelerator))
	}
	return result
}
