package workload

import "math"

type Rung struct {
	Width, Height int
	FrameRate     float64
}

func rungCost(rung Rung) float64 {
	frameRate := rung.FrameRate
	if math.IsNaN(frameRate) || frameRate <= 0 {
		frameRate = 30
	}
	frameRate = min(60, max(1, frameRate))
	return 2 * float64(max(0, rung.Width)) * float64(max(0, rung.Height)) * frameRate / (1920 * 1080 * 30)
}

func ladderUnits(rungs []Rung) float64 {
	total := 0.0
	for _, rung := range rungs {
		total += rungCost(rung)
	}
	return total
}

func (governor *Governor) LadderCost(rungs []Rung) int {
	capacity := governor.EncodingCapacity()
	return min(capacity, max(1, int(math.Ceil(ladderUnits(rungs)-1e-9))))
}

func (governor *Governor) LadderRungs(rungs []Rung) int {
	if len(rungs) == 0 {
		return 0
	}
	budget := max(rungCost(rungs[len(rungs)-1]), float64(governor.EncodingCapacity())/2)
	count := 1
	for count < len(rungs) && ladderUnits(rungs[len(rungs)-count-1:]) <= budget+1e-9 {
		count++
	}
	return count
}
