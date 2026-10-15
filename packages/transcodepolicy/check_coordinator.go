package transcodepolicy

import (
	"context"
	"sync"
)

type CheckCoordinatorDependencies struct {
	Lock           *sync.RWMutex
	Result         *CheckResult
	FFmpeg         string
	Resolve        func() (Settings, error)
	Pending        func() Settings
	BackendName    func(string) string
	RecordHardware func(Settings, CheckResult)
}

type CheckCoordinator struct {
	dependencies CheckCoordinatorDependencies
}

func NewCheckCoordinator(dependencies CheckCoordinatorDependencies) CheckCoordinator {
	return CheckCoordinator{dependencies: dependencies}
}

func (coordinator CheckCoordinator) Run(ctx context.Context) CheckResult {
	options, _ := coordinator.dependencies.Resolve()
	result := Check(ctx, coordinator.dependencies.FFmpeg, options, coordinator.dependencies.BackendName(options.Accelerator))
	coordinator.dependencies.RecordHardware(options, result)
	coordinator.dependencies.Lock.Lock()
	*coordinator.dependencies.Result = result
	coordinator.dependencies.Lock.Unlock()
	return result
}

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
