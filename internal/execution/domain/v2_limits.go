package domain

type V2Limits struct {
	MaxPublishedBytes int64
	MaxStoredBytes    int64
	MaxChunks         int64
	MaxNodes          int64
	MaxPendingTasks   int64
}

func DefaultV2Limits() V2Limits { return V2Limits{1 << 30, 8 << 30, 1000000, 1000000, 100000} }
