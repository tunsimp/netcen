package tcp

type DeviceID string

type DeviceRegistration struct {
	Token    string   `json:"token"`
	DeviceID DeviceID `json:"device_id"`
}

type ConflictResolution string

const (
	AcceptedLastWriteWins ConflictResolution = "accepted_last_write_wins"
	IgnoredStaleUpdate    ConflictResolution = "ignored_stale_update"
)

type ProgressUpdate struct {
	UserID    string `json:"user_id"`
	MangaID   string `json:"manga_id"`
	Chapter   int    `json:"chapter"`
	Status    string `json:"status"`
	Timestamp int64  `json:"timestamp"`
}

type ProgressSyncMessage struct {
	ProgressUpdate
	DeviceID           DeviceID           `json:"device_id"`
	ConflictResolution ConflictResolution `json:"conflict_resolution,omitempty"`
}
