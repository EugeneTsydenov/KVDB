package protocol

import (
	"testing"
)

func TestStatusValues(t *testing.T) {
	t.Parallel()

	statuses := []Status{
		StatusOK,
		StatusNotFound,
		StatusNotSet,
		StatusBadRequest,
		StatusUnknownOpCode,
		StatusUnsupportedVersion,
		StatusTooLarge,
		StatusInvalidArgument,
		StatusInternalError,
	}
	for _, status := range statuses {
		if !status.Valid() {
			t.Errorf("%v should be valid", status)
		}
		if status.String() == "" {
			t.Errorf("%d has an empty string representation", status)
		}
	}
	if Status(0xFF).Valid() || Status(0xFF).String() != "Status(0xFF)" {
		t.Error("unknown status handling is incorrect")
	}
}
