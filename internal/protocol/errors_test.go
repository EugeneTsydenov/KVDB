package protocol

import (
	"errors"
	"testing"
)

func TestStatusAndErrorHelpers(t *testing.T) {
	t.Parallel()

	statusCases := []struct {
		err  error
		want Status
	}{
		{err: nil, want: StatusOK},
		{err: ErrBadMagic, want: StatusBadRequest},
		{err: ErrBadLength, want: StatusBadRequest},
		{err: ErrBadFlags, want: StatusBadRequest},
		{err: ErrTruncated, want: StatusBadRequest},
		{err: ErrTrailingBytes, want: StatusBadRequest},
		{err: ErrUnsupportedVersion, want: StatusUnsupportedVersion},
		{err: ErrUnknownOpCode, want: StatusUnknownOpCode},
		{err: ErrTooLarge, want: StatusTooLarge},
		{err: ErrInvalidArgument, want: StatusInvalidArgument},
		{err: errors.New("unexpected"), want: StatusInternalError},
		{err: errors.Join(errors.New("context"), ErrTooLarge), want: StatusTooLarge},
	}

	for _, tc := range statusCases {
		if got := StatusOf(tc.err); got != tc.want {
			t.Errorf("StatusOf(%v) = %v, want %v", tc.err, got, tc.want)
		}
	}

	replyCases := []struct {
		err  error
		want bool
	}{
		{err: nil, want: true},
		{err: ErrBadMagic, want: false},
		{err: ErrBadLength, want: false},
		{err: ErrBadFlags, want: true},
		{err: ErrTruncated, want: true},
		{err: ErrInvalidArgument, want: true},
		{err: errors.Join(errors.New("context"), ErrBadMagic), want: false},
	}

	for _, tc := range replyCases {
		if got := ShouldReply(tc.err); got != tc.want {
			t.Errorf("ShouldReply(%v) = %v, want %v", tc.err, got, tc.want)
		}
	}
}
