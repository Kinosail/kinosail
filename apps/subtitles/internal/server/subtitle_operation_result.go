package server

import (
	"encoding/json"
	"math"
	"net/http"
)

func validSubtitleOperationAudio(data []byte) bool {
	var result struct {
		Duration float64   `json:"duration"`
		Waveform []float64 `json:"waveform"`
		Speech   []float64 `json:"speech"`
	}
	if len(data) > 128*1024 || json.Unmarshal(data, &result) != nil || !finiteSubtitleOperationNumber(result.Duration) || result.Duration <= 0 || result.Duration > subtitleAudioLimit.Seconds() {
		return false
	}
	for _, values := range [][]float64{result.Waveform, result.Speech} {
		if len(values) < 1 || len(values) > 1024 {
			return false
		}
		for _, value := range values {
			if !finiteSubtitleOperationNumber(value) {
				return false
			}
		}
	}
	return true
}

func finiteSubtitleOperationNumber(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func (operations *subtitleOperations) retainResult(id string, data []byte) {
	operations.results[id] = append([]byte(nil), data...)
	operations.resultOrder = append(operations.resultOrder, id)
	for len(operations.resultOrder) > 2 {
		delete(operations.results, operations.resultOrder[0])
		operations.resultOrder = operations.resultOrder[1:]
	}
}

func (operations *subtitleOperations) result(request *http.Request, id string) ([]byte, int) {
	operations.mu.Lock()
	defer operations.mu.Unlock()
	record, status := operations.record(request, id)
	if status != http.StatusOK {
		return nil, status
	}
	if record.State != "completed" || record.Action != "audio" || record.Outcome != "success" {
		return nil, http.StatusConflict
	}
	data, found := operations.results[id]
	if !found {
		return nil, http.StatusNotFound
	}
	return data, http.StatusOK
}
