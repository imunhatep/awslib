package acm

import (
	stderrors "errors"
	"testing"

	acmtypes "github.com/aws/aws-sdk-go-v2/service/acm/types"
	"github.com/go-errors/errors"
	"github.com/stretchr/testify/assert"
)

func TestClassifyErrNil(t *testing.T) {
	assert.NoError(t, classifyErr(nil))
}

// The whole point of the sentinels is that callers can branch with errors.Is
// after the error has been wrapped for a stack trace, so assert that the chain
// survives both the fmt wrap and the go-errors wrap.
func TestClassifyErrMapsToSentinels(t *testing.T) {
	tests := []struct {
		name     string
		apiErr   error
		expected error
	}{
		{"resource not found", &acmtypes.ResourceNotFoundException{}, ErrNotFound},
		{"resource in use", &acmtypes.ResourceInUseException{}, ErrInUse},
		{"invalid state", &acmtypes.InvalidStateException{}, ErrInvalidState},
		{"request in progress", &acmtypes.RequestInProgressException{}, ErrRequestInProgress},
		{"conflict", &acmtypes.ConflictException{}, ErrConflict},
		{"limit exceeded", &acmtypes.LimitExceededException{}, ErrLimitExceeded},
		{"service quota exceeded", &acmtypes.ServiceQuotaExceededException{}, ErrLimitExceeded},
		{"too many tags", &acmtypes.TooManyTagsException{}, ErrLimitExceeded},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := classifyErr(tt.apiErr)

			assert.Error(t, err)
			assert.True(t, stderrors.Is(err, tt.expected), "expected errors.Is to match %v", tt.expected)
			assert.True(t, errors.Is(err, tt.expected), "expected go-errors Is to match %v", tt.expected)
			assert.True(t, stderrors.Is(err, tt.apiErr), "original API error should stay reachable")
		})
	}
}

func TestClassifyErrLeavesUnknownErrorsUnmapped(t *testing.T) {
	for _, original := range []error{
		stderrors.New("something else went wrong"),
		&acmtypes.InvalidArnException{},
		&acmtypes.ThrottlingException{},
	} {
		err := classifyErr(original)

		assert.Error(t, err)
		assert.True(t, stderrors.Is(err, original))
		for _, sentinel := range []error{
			ErrNotFound, ErrInUse, ErrInvalidState,
			ErrRequestInProgress, ErrConflict, ErrLimitExceeded,
		} {
			assert.False(t, stderrors.Is(err, sentinel), "%T must not map to %v", original, sentinel)
		}
	}
}
