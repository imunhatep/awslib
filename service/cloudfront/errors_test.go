package cloudfront

import (
	stderrors "errors"
	"testing"

	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
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
		{"entity not found", &cftypes.EntityNotFound{}, ErrNotFound},
		{"no such resource", &cftypes.NoSuchResource{}, ErrNotFound},
		{"entity already exists", &cftypes.EntityAlreadyExists{}, ErrAlreadyExists},
		{"cname already exists", &cftypes.CNAMEAlreadyExists{}, ErrAlreadyExists},
		{"precondition failed", &cftypes.PreconditionFailed{}, ErrPreconditionFailed},
		{"invalid if-match version", &cftypes.InvalidIfMatchVersion{}, ErrPreconditionFailed},
		{"resource not disabled", &cftypes.ResourceNotDisabled{}, ErrNotDisabled},
		{"resource in use", &cftypes.ResourceInUse{}, ErrInUse},
		{"entity limit exceeded", &cftypes.EntityLimitExceeded{}, ErrLimitExceeded},
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
	original := stderrors.New("something else went wrong")

	err := classifyErr(original)

	assert.Error(t, err)
	assert.True(t, stderrors.Is(err, original))
	for _, sentinel := range []error{
		ErrNotFound, ErrAlreadyExists, ErrPreconditionFailed,
		ErrNotDisabled, ErrInUse, ErrLimitExceeded,
	} {
		assert.False(t, stderrors.Is(err, sentinel))
	}
}
