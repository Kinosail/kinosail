package server

import "github.com/MikeO7/kinosail/packages/downloads"

type (
	downloadJob     = downloads.Job
	downloadManager struct{ *downloads.Manager }
)

const downloadQueueCapacity = downloads.QueueCapacity
