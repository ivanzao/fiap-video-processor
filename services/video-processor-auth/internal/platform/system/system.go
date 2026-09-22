package system

import (
	"time"

	"github.com/google/uuid"
)

type Clock struct{}

func (Clock) Now() time.Time { return time.Now() }

type UUIDGenerator struct{}

func (UUIDGenerator) NewID() string { return uuid.NewString() }
