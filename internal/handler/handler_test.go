package handler

import (
	"encoding/binary"
	"errors"
	"fmt"
	"testing"
	"time"

	"kvdb/internal/protocol"
	"kvdb/internal/storage"
)

type recordingStore struct {
	calls []string

	key     string
	value   storage.Value
	ttl     time.Duration
	pattern string

	getValue  storage.Value
	getErr    error
	setErr    error
	setNXOK   bool
	setNXErr  error
	delOK     bool
	ttlValue  time.Duration
	ttlErr    error
	expireErr error
	keys      []string
	keysErr   error
}

func (s *recordingStore) Get(key string) (storage.Value, error) {
	s.calls = append(s.calls, "GET")
	s.key = key
	return s.getValue, s.getErr
}
func (s *recordingStore) Set(key string, value storage.Value, ttl time.Duration) error {
	s.calls = append(s.calls, "SET")
	s.key, s.value, s.ttl = key, value.Clone(), ttl
	return s.setErr
}
func (s *recordingStore) SetNX(key string, value storage.Value, ttl time.Duration) (bool, error) {
	s.calls = append(s.calls, "SETNX")
	s.key, s.value, s.ttl = key, value.Clone(), ttl
	return s.setNXOK, s.setNXErr
}
func (s *recordingStore) Del(key string) bool {
	s.calls = append(s.calls, "DEL")
	s.key = key
	return s.delOK
}
func (s *recordingStore) TTL(key string) (time.Duration, error) {
	s.calls = append(s.calls, "TTL")
	s.key = key
	return s.ttlValue, s.ttlErr
}
func (s *recordingStore) Expire(key string, ttl time.Duration) error {
	s.calls = append(s.calls, "EXPIRE")
	s.key, s.ttl = key, ttl
	return s.expireErr
}
func (s *recordingStore) Keys(pattern string) ([]string, error) {
	s.calls = append(s.calls, "KEYS")
	s.pattern = pattern
	return s.keys, s.keysErr
}

func decodeResult(t *testing.T, h *Handler, req protocol.Request) protocol.Response {
	t.Helper()
	packet, err := protocol.EncodeRequest(req)
	if err != nil {
		t.Fatalf("EncodeRequest: %v", err)
	}
	responsePacket, err := h.Handle(packet)
	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if responsePacket == nil {
		t.Fatal("Handle returned nil response")
	}
	resp, err := protocol.DecodeResponse(responsePacket)
	if err != nil {
		t.Fatalf("DecodeResponse: %v", err)
	}
	return resp
}

func TestNewRejectsNilStore(t *testing.T) {
	if _, err := New(nil); err == nil {
		t.Fatal("New accepted a nil Store")
	}
}

func TestHandlerDispatchesCommandsAndPreservesResponseFields(t *testing.T) {
	const requestID = 0x1020304050607080
	cases := []struct {
		name         string
		req          protocol.Request
		store        *recordingStore
		wantCall     string
		wantStatus   protocol.Status
		wantValue    string
		wantTTL      int64
		wantDel      uint64
		wantKeys     []string
		wantKey      string
		wantValueArg string
		wantPattern  string
		wantStoreTTL time.Duration
	}{
		{
			name: "PING", req: protocol.Request{Op: protocol.OpPing, RequestID: requestID},
			store: &recordingStore{}, wantStatus: protocol.StatusOK,
		},
		{
			name: "GET", req: protocol.Request{Op: protocol.OpGet, RequestID: requestID, Key: []byte("alpha")},
			store:    &recordingStore{getValue: storage.NewStringValue([]byte("payload"))},
			wantCall: "GET", wantStatus: protocol.StatusOK, wantValue: "payload", wantKey: "alpha",
		},
		{
			name: "SET", req: protocol.Request{Op: protocol.OpSet, RequestID: requestID, Key: []byte("alpha"), Value: []byte("payload"), TTLMs: 1250},
			store: &recordingStore{}, wantCall: "SET", wantStatus: protocol.StatusOK,
			wantKey: "alpha", wantValueArg: "payload", wantStoreTTL: 1250 * time.Millisecond,
		},
		{
			name: "SETNX created", req: protocol.Request{Op: protocol.OpSetNX, RequestID: requestID, Key: []byte("new"), Value: []byte("v"), TTLMs: 250},
			store: &recordingStore{setNXOK: true}, wantCall: "SETNX", wantStatus: protocol.StatusOK,
			wantKey: "new", wantValueArg: "v", wantStoreTTL: 250 * time.Millisecond,
		},
		{
			name: "SETNX existing", req: protocol.Request{Op: protocol.OpSetNX, RequestID: requestID, Key: []byte("existing"), Value: []byte("ignored")},
			store: &recordingStore{setNXOK: false}, wantCall: "SETNX", wantStatus: protocol.StatusNotSet,
			wantKey: "existing", wantValueArg: "ignored",
		},
		{
			name: "DEL", req: protocol.Request{Op: protocol.OpDel, RequestID: requestID, Key: []byte("alpha")},
			store: &recordingStore{delOK: true}, wantCall: "DEL", wantStatus: protocol.StatusOK, wantDel: 1, wantKey: "alpha",
		},
		{
			name: "TTL without expiration", req: protocol.Request{Op: protocol.OpTTL, RequestID: requestID, Key: []byte("alpha")},
			store: &recordingStore{ttlErr: storage.ErrKeyNoTTL}, wantCall: "TTL", wantStatus: protocol.StatusOK, wantTTL: -1, wantKey: "alpha",
		},
		{
			name: "TTL positive", req: protocol.Request{Op: protocol.OpTTL, RequestID: requestID, Key: []byte("alpha")},
			store: &recordingStore{ttlValue: 1200 * time.Millisecond}, wantCall: "TTL", wantStatus: protocol.StatusOK, wantTTL: 1200, wantKey: "alpha",
		},
		{
			name: "EXPIRE", req: protocol.Request{Op: protocol.OpExpire, RequestID: requestID, Key: []byte("alpha"), TTLMs: 1500},
			store: &recordingStore{}, wantCall: "EXPIRE", wantStatus: protocol.StatusOK, wantKey: "alpha", wantStoreTTL: 1500 * time.Millisecond,
		},
		{
			name: "KEYS", req: protocol.Request{Op: protocol.OpKeys, RequestID: requestID, Pattern: "^user"},
			store: &recordingStore{keys: []string{"user:1", "user:2"}}, wantCall: "KEYS", wantStatus: protocol.StatusOK,
			wantKeys: []string{"user:1", "user:2"}, wantPattern: "^user",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, err := New(tc.store)
			if err != nil {
				t.Fatal(err)
			}
			resp := decodeResult(t, h, tc.req)
			if resp.Op != tc.req.Op || resp.RequestID != requestID || resp.Status != tc.wantStatus {
				t.Fatalf("response header = {op:%s id:%d status:%s}, want {%s %d %s}", resp.Op, resp.RequestID, resp.Status, tc.req.Op, requestID, tc.wantStatus)
			}
			if string(resp.Value) != tc.wantValue || resp.TTLMs != tc.wantTTL || resp.Deleted != tc.wantDel {
				t.Fatalf("response payload = {value:%q ttl:%d deleted:%d}", resp.Value, resp.TTLMs, resp.Deleted)
			}
			if fmt.Sprint(resp.Keys) != fmt.Sprint(tc.wantKeys) {
				t.Fatalf("keys=%v want %v", resp.Keys, tc.wantKeys)
			}
			if tc.wantCall == "" {
				if len(tc.store.calls) != 0 {
					t.Fatalf("PING called Store methods: %v", tc.store.calls)
				}
				return
			}
			if len(tc.store.calls) != 1 || tc.store.calls[0] != tc.wantCall {
				t.Fatalf("Store calls=%v want [%s]", tc.store.calls, tc.wantCall)
			}
			if tc.store.key != tc.wantKey || tc.store.pattern != tc.wantPattern || string(tc.store.value.Data) != tc.wantValueArg || tc.store.ttl != tc.wantStoreTTL {
				t.Fatalf("Store args: key=%q pattern=%q value=%q ttl=%s", tc.store.key, tc.store.pattern, tc.store.value.Data, tc.store.ttl)
			}
		})
	}
}

func TestHandlerMapsStorageErrorsToStatuses(t *testing.T) {
	boom := errors.New("store unavailable")
	cases := []struct {
		name  string
		req   protocol.Request
		store *recordingStore
		want  protocol.Status
	}{
		{"GET missing", protocol.Request{Op: protocol.OpGet, Key: []byte("k")}, &recordingStore{getErr: storage.ErrEntryNotFound}, protocol.StatusNotFound},
		{"TTL missing", protocol.Request{Op: protocol.OpTTL, Key: []byte("k")}, &recordingStore{ttlErr: storage.ErrEntryNotFound}, protocol.StatusNotFound},
		{"EXPIRE missing", protocol.Request{Op: protocol.OpExpire, Key: []byte("k"), TTLMs: 1}, &recordingStore{expireErr: storage.ErrEntryNotFound}, protocol.StatusNotFound},
		{"SET invalid TTL", protocol.Request{Op: protocol.OpSet, Key: []byte("k"), Value: []byte("v")}, &recordingStore{setErr: storage.ErrTTLInvalid}, protocol.StatusInvalidArgument},
		{"KEYS invalid regex", protocol.Request{Op: protocol.OpKeys, Pattern: "["}, nil, protocol.StatusInvalidArgument},
		{"SET internal", protocol.Request{Op: protocol.OpSet, Key: []byte("k"), Value: []byte("v")}, &recordingStore{setErr: boom}, protocol.StatusInternalError},
		{"SETNX internal", protocol.Request{Op: protocol.OpSetNX, Key: []byte("k"), Value: []byte("v")}, &recordingStore{setNXErr: boom}, protocol.StatusInternalError},
		{"KEYS internal", protocol.Request{Op: protocol.OpKeys, Pattern: ".*"}, &recordingStore{keysErr: boom}, protocol.StatusInternalError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := tc.store
			if store == nil {
				t.Skip("invalid regex is covered by malformed-packet test")
			}
			h, err := New(store)
			if err != nil {
				t.Fatal(err)
			}
			resp := decodeResult(t, h, tc.req)
			if resp.Status != tc.want {
				t.Fatalf("status=%s want %s (message=%q)", resp.Status, tc.want, resp.Message)
			}
			if tc.want != protocol.StatusNotFound && resp.Message == "" {
				t.Fatal("error response has no message")
			}
		})
	}
}

func setPayloadLen(packet []byte, n int) []byte {
	binary.BigEndian.PutUint16(packet[6:8], uint16(n))
	return packet
}

func TestHandlerDecodeErrorsAndReplyPolicy(t *testing.T) {
	base, err := protocol.EncodeRequest(protocol.Request{Op: protocol.OpPing, RequestID: 91})
	if err != nil {
		t.Fatal(err)
	}
	badMagic := bytesClone(base)
	badMagic[0] ^= 0xff
	badVersion := bytesClone(base)
	badVersion[2]++
	badFlags := bytesClone(base)
	badFlags[5] = 1
	unknownOp := bytesClone(base)
	unknownOp[3] = 0xff
	badPacketLength := bytesClone(base)
	setPayloadLen(badPacketLength, 1)
	trailing := append(bytesClone(base), 0x01)
	setPayloadLen(trailing, 1)
	truncated := bytesClone(base[:protocol.HeaderSize])
	truncated[3] = byte(protocol.OpGet)
	truncated = append(truncated, 0x00)
	setPayloadLen(truncated, 1)
	invalidRegexp, err := protocol.EncodeRequest(protocol.Request{Op: protocol.OpKeys, RequestID: 92, Pattern: "x"})
	if err != nil {
		t.Fatal(err)
	}
	invalidRegexp[18] = '['
	oversizedKey, err := protocol.EncodeRequest(protocol.Request{Op: protocol.OpGet, RequestID: 93, Key: []byte("x")})
	if err != nil {
		t.Fatal(err)
	}
	binary.BigEndian.PutUint16(oversizedKey[16:18], protocol.MaxKeyLen+1)

	cases := []struct {
		name       string
		packet     []byte
		wantReply  bool
		wantStatus protocol.Status
	}{
		{"bad magic", badMagic, false, protocol.StatusBadRequest},
		{"unsupported version", badVersion, false, protocol.StatusUnsupportedVersion},
		{"wrong packet length", badPacketLength, false, protocol.StatusBadRequest},
		{"bad flags", badFlags, true, protocol.StatusBadRequest},
		{"unknown opcode", unknownOp, true, protocol.StatusUnknownOpCode},
		{"trailing payload bytes", trailing, true, protocol.StatusBadRequest},
		{"truncated payload field", truncated, true, protocol.StatusBadRequest},
		{"invalid regexp", invalidRegexp, true, protocol.StatusInvalidArgument},
		{"oversized key field", oversizedKey, true, protocol.StatusTooLarge},
	}

	h, err := New(&recordingStore{})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			responsePacket, err := h.Handle(tc.packet)
			if err != nil {
				t.Fatalf("Handle: %v", err)
			}
			if !tc.wantReply {
				if responsePacket != nil {
					t.Fatalf("expected no response, got %x", responsePacket)
				}
				return
			}
			if responsePacket == nil {
				t.Fatal("expected an error response")
			}
			resp, err := protocol.DecodeResponse(responsePacket)
			if err != nil {
				t.Fatalf("DecodeResponse: %v", err)
			}
			if resp.Status != tc.wantStatus {
				t.Fatalf("status=%s want %s (message=%q)", resp.Status, tc.wantStatus, resp.Message)
			}
		})
	}
}

func bytesClone(src []byte) []byte { return append([]byte(nil), src...) }

func TestProtocolDecodeErrorClassification(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantStatus protocol.Status
		wantReply  bool
	}{
		{"bad magic", protocol.ErrBadMagic, protocol.StatusBadRequest, false},
		{"bad length", protocol.ErrBadLength, protocol.StatusBadRequest, false},
		{"bad flags", protocol.ErrBadFlags, protocol.StatusBadRequest, true},
		{"truncated", protocol.ErrTruncated, protocol.StatusBadRequest, true},
		{"trailing bytes", protocol.ErrTrailingBytes, protocol.StatusBadRequest, true},
		{"unsupported version", protocol.ErrUnsupportedVersion, protocol.StatusUnsupportedVersion, true},
		{"unknown opcode", protocol.ErrUnknownOpCode, protocol.StatusUnknownOpCode, true},
		{"too large", protocol.ErrTooLarge, protocol.StatusTooLarge, true},
		{"invalid argument", protocol.ErrInvalidArgument, protocol.StatusInvalidArgument, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := protocol.StatusOf(tc.err); got != tc.wantStatus {
				t.Fatalf("StatusOf=%s want %s", got, tc.wantStatus)
			}
			if got := protocol.ShouldReply(tc.err); got != tc.wantReply {
				t.Fatalf("ShouldReply=%v want %v", got, tc.wantReply)
			}
		})
	}
}
