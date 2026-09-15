package procinfo

func CPUPercent(prevUTime, prevSTime, currUTime, currSTime uint64, elapsedSeconds float64) float64 {
	if elapsedSeconds <= 0 {
		return 0
	}
	if currUTime < prevUTime || currSTime < prevSTime {
		return 0
	}

	deltaNs := float64((currUTime - prevUTime) + (currSTime - prevSTime))
	deltaSeconds := deltaNs / 1e9

	return (deltaSeconds / elapsedSeconds) * 100.0
}