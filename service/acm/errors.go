package acm

import (
	stderrors "errors"
	"fmt"

	acmtypes "github.com/aws/aws-sdk-go-v2/service/acm/types"
	"github.com/go-errors/errors"
)

// Sentinel errors for the API conditions callers actually need to branch on.
// ACM reports these as distinct modelled error shapes; collapsing them into
// sentinels lets consumers use errors.Is instead of type-switching on SDK types
// or matching strings.
var (
	// ErrNotFound means the certificate does not exist (or was already deleted).
	ErrNotFound = stderrors.New("acm: certificate not found")

	// ErrInUse means a resource still references the certificate, e.g. a
	// CloudFront distribution tenant. ACM refuses to delete it until the
	// reference is gone.
	ErrInUse = stderrors.New("acm: certificate is still in use")

	// ErrInvalidState means the certificate is in a state that does not allow
	// the operation.
	ErrInvalidState = stderrors.New("acm: certificate is in an invalid state for the operation")

	// ErrRequestInProgress means ACM is still processing an earlier request for
	// the same certificate. Retry later.
	ErrRequestInProgress = stderrors.New("acm: request still in progress")

	// ErrConflict means a concurrent operation on the same certificate won.
	ErrConflict = stderrors.New("acm: conflicting operation in progress")

	// ErrLimitExceeded means an account or per-certificate quota was hit: the
	// certificate count, the yearly issuance quota or the tag limit.
	ErrLimitExceeded = stderrors.New("acm: quota exceeded")
)

// classifyErr maps an ACM API error onto a package sentinel where one applies,
// preserving the original error for context, and wraps everything with a stack
// trace. Returns nil for a nil error.
func classifyErr(err error) error {
	if err == nil {
		return nil
	}

	if sentinel := matchSentinel(err); sentinel != nil {
		return errors.New(fmt.Errorf("%w: %w", sentinel, err))
	}

	return errors.New(err)
}

func matchSentinel(err error) error {
	var (
		resourceNotFound *acmtypes.ResourceNotFoundException

		resourceInUse *acmtypes.ResourceInUseException

		invalidState *acmtypes.InvalidStateException

		requestInProgress *acmtypes.RequestInProgressException

		conflict *acmtypes.ConflictException

		limitExceeded        *acmtypes.LimitExceededException
		serviceQuotaExceeded *acmtypes.ServiceQuotaExceededException
		tooManyTags          *acmtypes.TooManyTagsException
	)

	switch {
	case stderrors.As(err, &resourceNotFound):
		return ErrNotFound

	case stderrors.As(err, &resourceInUse):
		return ErrInUse

	case stderrors.As(err, &invalidState):
		return ErrInvalidState

	case stderrors.As(err, &requestInProgress):
		return ErrRequestInProgress

	case stderrors.As(err, &conflict):
		return ErrConflict

	case stderrors.As(err, &limitExceeded),
		stderrors.As(err, &serviceQuotaExceeded),
		stderrors.As(err, &tooManyTags):
		return ErrLimitExceeded
	}

	return nil
}
