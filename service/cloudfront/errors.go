package cloudfront

import (
	stderrors "errors"
	"fmt"

	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/go-errors/errors"
)

// Sentinel errors for the API conditions callers actually need to branch on.
// CloudFront reports these as distinct modelled error shapes; collapsing them
// into sentinels lets consumers use errors.Is instead of type-switching on SDK
// types or matching strings.
var (
	// ErrNotFound covers a missing tenant, connection group or distribution.
	ErrNotFound = stderrors.New("cloudfront: resource not found")

	// ErrAlreadyExists is returned when a tenant name or domain is taken.
	ErrAlreadyExists = stderrors.New("cloudfront: resource already exists")

	// ErrPreconditionFailed means the supplied ETag was stale — the resource
	// changed underneath the caller. Re-read and retry.
	ErrPreconditionFailed = stderrors.New("cloudfront: precondition failed, stale ETag")

	// ErrNotDisabled means a delete was attempted on an enabled resource.
	// CloudFront requires disable-then-delete.
	ErrNotDisabled = stderrors.New("cloudfront: resource must be disabled before deletion")

	// ErrInUse means the resource is still referenced by something else, e.g. a
	// connection group that still has tenants attached.
	ErrInUse = stderrors.New("cloudfront: resource is still in use")

	// ErrLimitExceeded means an account or per-resource quota was hit.
	ErrLimitExceeded = stderrors.New("cloudfront: quota exceeded")
)

// classifyErr maps a CloudFront API error onto a package sentinel where one
// applies, preserving the original error for context, and wraps everything with
// a stack trace. Returns nil for a nil error.
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
		entityNotFound     *cftypes.EntityNotFound
		noSuchResource     *cftypes.NoSuchResource
		noSuchDistribution *cftypes.NoSuchDistribution

		entityAlreadyExists *cftypes.EntityAlreadyExists
		cnameAlreadyExists  *cftypes.CNAMEAlreadyExists

		preconditionFailed    *cftypes.PreconditionFailed
		invalidIfMatchVersion *cftypes.InvalidIfMatchVersion

		resourceNotDisabled     *cftypes.ResourceNotDisabled
		distributionNotDisabled *cftypes.DistributionNotDisabled

		resourceInUse            *cftypes.ResourceInUse
		cannotDeleteWhileInUse   *cftypes.CannotDeleteEntityWhileInUse
		cannotUpdateWhileInUse   *cftypes.CannotUpdateEntityWhileInUse
		entityLimitExceeded      *cftypes.EntityLimitExceeded
		tooManyDistributionCNAME *cftypes.TooManyDistributionCNAMEs
	)

	switch {
	case stderrors.As(err, &entityNotFound),
		stderrors.As(err, &noSuchResource),
		stderrors.As(err, &noSuchDistribution):
		return ErrNotFound

	case stderrors.As(err, &entityAlreadyExists),
		stderrors.As(err, &cnameAlreadyExists):
		return ErrAlreadyExists

	case stderrors.As(err, &preconditionFailed),
		stderrors.As(err, &invalidIfMatchVersion):
		return ErrPreconditionFailed

	case stderrors.As(err, &resourceNotDisabled),
		stderrors.As(err, &distributionNotDisabled):
		return ErrNotDisabled

	case stderrors.As(err, &resourceInUse),
		stderrors.As(err, &cannotDeleteWhileInUse),
		stderrors.As(err, &cannotUpdateWhileInUse):
		return ErrInUse

	case stderrors.As(err, &entityLimitExceeded),
		stderrors.As(err, &tooManyDistributionCNAME):
		return ErrLimitExceeded
	}

	return nil
}
