package sqlite

import "time"

func memoryTimestamp() time.Time {
	return time.Now().UTC()
}
