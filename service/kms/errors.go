package kms

import (
	stderrors "errors"
	"fmt"

	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
	"github.com/go-errors/errors"
)

// ErrNotFound means the key does not exist: it was deleted, or never existed
// in this account and region. A key pending deletion still exists.
var ErrNotFound = stderrors.New("kms: key not found")

// classifyErr maps a KMS API error onto ErrNotFound where it applies,
// preserving the original error for context, and wraps everything with a stack
// trace. Returns nil for a nil error.
func classifyErr(err error) error {
	if err == nil {
		return nil
	}

	var notFound *kmstypes.NotFoundException
	if stderrors.As(err, &notFound) {
		return errors.New(fmt.Errorf("%w: %w", ErrNotFound, err))
	}

	return errors.New(err)
}
