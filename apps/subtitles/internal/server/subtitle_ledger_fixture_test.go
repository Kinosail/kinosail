package server

import "os"

func completeSubtitleRecord(path string, data []byte, record subtitleRecord) subtitleRecord {
	record.Fingerprint = subtitleFingerprint(data)
	if info, err := os.Stat(path); err == nil {
		record.Size, record.Modified = info.Size(), info.ModTime().UnixNano()
	}
	return record
}
