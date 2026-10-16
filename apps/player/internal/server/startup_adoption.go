package server

func (encoding *startupEncoding) adopt() {
	encoding.adopted.Store(true)
	signal := encoding.adoption()
	encoding.adoptedMu.Lock()
	select {
	case <-signal:
	default:
		close(signal)
	}
	encoding.adoptedMu.Unlock()
}

func (encoding *startupEncoding) adoption() chan struct{} {
	encoding.adoptedMu.Lock()
	defer encoding.adoptedMu.Unlock()
	if encoding.adoptedSignal == nil {
		encoding.adoptedSignal = make(chan struct{})
	}
	return encoding.adoptedSignal
}
