package errors

import (
	"errors"
)

var (
	// ErrNotSupport not support this function
	ErrNotSupport = errors.New("not support this function yet")
	// ErrCanceled canceled
	ErrCanceled = errors.New("the context had been canceled")
	// ErrFailedToListen cannot listen on an address, or bad port
	ErrFailedToListen = errors.New("failed to listen, bad port")
	// ErrBadRequest bad request
	ErrBadRequest = errors.New("bad request")
	// ErrBadMetadata bad metadata
	ErrBadMetadata = errors.New("bad metadata")
	// ErrConnectionChClosed connection channel been closed
	ErrConnectionChClosed = errors.New("connection channel already closed")
	// ErrMsgChanClosed message type channel been closed
	ErrMsgChanClosed = errors.New("message type channel already closed")
	// ErrFailedToAllocatePort failed to allocate port
	ErrFailedToAllocatePort = errors.New("failed to allocate port for user")
	// ErrTokenNotValid token not valid
	ErrTokenNotValid = errors.New("token not valid")
	// ErrFailedToRegisterAddr failed to register addr
	ErrFailedToRegisterAddr = errors.New("failed to register addr")
)
