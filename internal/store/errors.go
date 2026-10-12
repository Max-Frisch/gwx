package store

import "errors"

var (
	ErrNotFound              = errors.New("resource not found")
	ErrUserExists            = errors.New("username or email already registered")
	ErrInvalidCredentials    = errors.New("invalid username or password")
	ErrInsufficientResources = errors.New("insufficient resources")
	ErrInsufficientShips     = errors.New("insufficient ships")
	ErrInsufficientFuel      = errors.New("insufficient hydrogen")
	ErrQueueBusy             = errors.New("build queue is busy")
	ErrPrerequisites         = errors.New("prerequisites not met")
	ErrUnknownCode           = errors.New("unknown build code")
	ErrInvalidQuantity       = errors.New("invalid quantity")
	ErrNoFields              = errors.New("not enough fields")
	ErrTargetOccupied        = errors.New("target coordinates are occupied")
	ErrInvalidCoordinates    = errors.New("coordinates out of range")
	ErrCannotRecall          = errors.New("fleet cannot be recalled")
	ErrNoTarget              = errors.New("mission requires an existing target")
	ErrNotOwner              = errors.New("not the owner of this planet")
	ErrAllianceTagTaken      = errors.New("alliance tag already taken")
	ErrAlreadyMember         = errors.New("already in an alliance")
	ErrNotMember             = errors.New("not a member of that alliance")
	ErrAllianceClosed        = errors.New("alliance is closed to direct joins")
	ErrForbidden             = errors.New("insufficient alliance rights")
)
