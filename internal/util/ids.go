// internal/util/ids.go
// Generator ID sederhana untuk request/audit

package util

import (
	"github.com/google/uuid"
)

func NewID() string {
	return uuid.New().String()
}
