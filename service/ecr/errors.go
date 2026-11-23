package ecr

import (
	stderrors "errors"
	"fmt"

	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
	"github.com/go-errors/errors"
)

// ErrNotFound means the repository does not exist (or was already deleted).
var ErrNotFound = stderrors.New("ecr: repository not found")

// classifyErr maps an ECR API error onto ErrNotFound where it applies,
// preserving the original error for context, and wraps everything with a stack
// trace. Returns nil for a nil error.
func classifyErr(err error) error {
	if err == nil {
		return nil
	}

	var notFound *ecrtypes.RepositoryNotFoundException
	if stderrors.As(err, &notFound) {
		return errors.New(fmt.Errorf("%w: %w", ErrNotFound, err))
	}

	return errors.New(err)
}
