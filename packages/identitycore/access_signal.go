package identitycore

import "sync"

type AccessSignal struct {
	mu   sync.Mutex
	next chan struct{}
}

func (signal *AccessSignal) Changed() <-chan struct{} {
	signal.mu.Lock()
	defer signal.mu.Unlock()
	if signal.next == nil {
		signal.next = make(chan struct{})
	}
	return signal.next
}

func (signal *AccessSignal) Notify() {
	signal.mu.Lock()
	defer signal.mu.Unlock()
	if signal.next != nil {
		close(signal.next)
		signal.next = nil
	}
}

type NotifyingBatchStore struct {
	Store  ProfileBatchStore
	Signal *AccessSignal
}

func (store NotifyingBatchStore) SaveJSONBatch(values map[string]any) error {
	defer store.Signal.Notify()
	return store.Store.SaveJSONBatch(values)
}

func NotifyingPersist(persist func(string, any) error, signal *AccessSignal) func(string, any) error {
	return func(path string, value any) error {
		defer signal.Notify()
		return persist(path, value)
	}
}
