package domain

import "time"

type V2State struct {
	ResultsEnabled     bool      `json:"resultsEnabled"`
	ResultCount        int64     `json:"resultCount"`
	LastResultSequence string    `json:"lastResultSequence"`
	CheckpointVersion  int       `json:"checkpointVersion"`
	DeadlineAt         time.Time `json:"deadlineAt"`
	Generation         int64     `json:"-"`
	Recoveries         int64     `json:"recoveries"`
	LastSequence       string    `json:"lastSequence"`
	PublishedBytes     int64     `json:"publishedBytes"`
	StoredBytes        int64     `json:"storedBytes"`
	LogicalRequests    int64     `json:"logicalRequests"`
	HTTPAttempts       int64     `json:"httpAttempts"`
	PendingTasks       int64     `json:"pendingTasks"`
	Terminal           bool      `json:"terminal"`
}
