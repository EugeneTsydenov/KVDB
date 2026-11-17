package handler

import (
	"errors"
	"kvdb/internal/protocol"
	"kvdb/internal/storage"
	"time"
)

// Store defines the storage operations required by Handler.
type Store interface {
	Get(key string) (storage.Value, error)
	Set(key string, value storage.Value, ttl time.Duration) error
	SetNX(key string, value storage.Value, ttl time.Duration) (bool, error)
	Del(key string) bool
	TTL(key string) (time.Duration, error)
	Expire(key string, ttl time.Duration) error
	Keys(pattern string) ([]string, error)
}

var _ Store = (*storage.MemoryStore)(nil)

// Handler decodes requests and dispatches commands to storage.
type Handler struct {
	store Store
}

func New(store Store) (*Handler, error) {
	if store == nil {
		return nil, errors.New("store must not be nil")
	}

	return &Handler{store: store}, nil
}

// Handle processes a binary request and returns an encoded response.
func (h *Handler) Handle(packet []byte) ([]byte, error) {
	req, err := protocol.DecodeRequest(packet)
	if err != nil {
		return h.handleDecodeError(packet, err)
	}

	resp := h.execute(req)

	return protocol.EncodeResponse(resp)
}

// handleDecodeError converts protocol errors into responses.
func (h *Handler) handleDecodeError(
	packet []byte,
	decodeErr error,
) ([]byte, error) {
	if !protocol.ShouldReply(decodeErr) {
		return nil, nil
	}

	header, err := protocol.DecodeHeader(packet)
	if err != nil {
		return nil, nil
	}

	resp := protocol.Response{
		Op:        header.Op,
		RequestID: header.RequestID,
		Status:    protocol.StatusOf(decodeErr),
		Message:   decodeErr.Error(),
	}

	return protocol.EncodeResponse(resp)
}

// execute dispatches the request to the appropriate storage operation.
func (h *Handler) execute(req protocol.Request) protocol.Response {
	resp := protocol.Response{
		Op:        req.Op,
		RequestID: req.RequestID,
		Status:    protocol.StatusOK,
	}

	key := string(req.Key)
	ttl := time.Duration(req.TTLMs) * time.Millisecond

	var err error

	switch req.Op {
	case protocol.OpPing:
		// No storage operation required.

	case protocol.OpGet:
		var value storage.Value
		value, err = h.store.Get(key)
		if err == nil {
			resp.Value = value.Data
		}

	case protocol.OpSet:
		value := storage.NewStringValue(req.Value)
		err = h.store.Set(key, value, ttl)

	case protocol.OpSetNX:
		value := storage.NewStringValue(req.Value)

		var created bool
		created, err = h.store.SetNX(key, value, ttl)
		if err == nil && !created {
			resp.Status = protocol.StatusNotSet
		}

	case protocol.OpDel:
		if h.store.Del(key) {
			resp.Deleted = 1
		}

	case protocol.OpTTL:
		var remaining time.Duration
		remaining, err = h.store.TTL(key)

		switch {
		case errors.Is(err, storage.ErrKeyNoTTL):
			resp.TTLMs = -1
			err = nil

		case err == nil:
			if remaining > 0 {
				resp.TTLMs = int64(remaining / time.Millisecond)
				if remaining%time.Millisecond != 0 {
					resp.TTLMs++
				}
			}
		}

	case protocol.OpExpire:
		err = h.store.Expire(key, ttl)

	case protocol.OpKeys:
		resp.Keys, err = h.store.Keys(req.Pattern)

	default:
		resp.Status = protocol.StatusUnknownOpCode
		resp.Message = protocol.ErrUnknownOpCode.Error()

		return resp
	}

	if err != nil {
		resp.Status = statusOfStorageError(err)
		resp.Message = err.Error()
	}

	return resp
}

// statusOfStorageError maps storage errors to protocol statuses.
func statusOfStorageError(err error) protocol.Status {
	switch {
	case errors.Is(err, storage.ErrEntryNotFound):
		return protocol.StatusNotFound

	case errors.Is(err, storage.ErrTTLInvalid),
		errors.Is(err, storage.ErrInvalidPattern):
		return protocol.StatusInvalidArgument

	default:
		return protocol.StatusInternalError
	}
}
