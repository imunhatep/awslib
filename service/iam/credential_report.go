package iam

import (
	"encoding/csv"
	stderrors "errors"
	"io"
	"strings"
	"time"

	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	awsiam "github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	"github.com/rs/zerolog/log"
)

// The IAM credential report is the cheap way to learn when a user's static credentials
// were last used.
//
// The per-user alternative is ListAccessKeys plus GetAccessKeyLastUsed for each key it
// returns, which at a few hundred users is several hundred calls per account per cycle,
// all against IAM's single global endpoint. The report is two calls for the whole
// account and carries the same last-used dates.
//
// What it costs in exchange: the report is regenerated at most every four hours, so the
// dates can lag by that much, and it carries no access key IDs. The lag does not matter
// to a caller measuring dormancy in months, and a caller that needs key IDs — to delete
// them — needs them only for the few users it is actually acting on, where ListAccessKeys
// is the right call.

// credentialReportPollInterval and credentialReportPollAttempts bound the wait for a
// report AWS is still building. Generation is fast in practice; the bound exists so a
// stuck report fails the call rather than hanging the read cycle.
const (
	credentialReportPollInterval = 2 * time.Second
	credentialReportPollAttempts = 10
)

// CredentialReportRootUser is the user column value AWS uses for the account root.
//
// The root row is not an IAM user: it has no UserName, cannot be tagged, and cannot be
// deleted. Any caller acting on report rows has to skip it.
const CredentialReportRootUser = "<root_account>"

// CredentialReportEntry is one row of the credential report.
//
// Times are pointers because the report expresses "never" and "not applicable" as
// non-dates (N/A, no_information, not_supported), and those must not collapse to the
// zero time — a nil LastUsed means the credential was never used, which is a different
// and stronger statement than "used long ago".
type CredentialReportEntry struct {
	User                  string
	Arn                   string
	UserCreationTime      *time.Time
	PasswordEnabled       bool
	PasswordLastUsed      *time.Time
	PasswordLastChanged   *time.Time
	MfaActive             bool
	AccessKey1Active      bool
	AccessKey1LastRotated *time.Time
	AccessKey1LastUsed    *time.Time
	AccessKey2Active      bool
	AccessKey2LastRotated *time.Time
	AccessKey2LastUsed    *time.Time
}

// IsRoot reports whether this row describes the account root rather than an IAM user.
func (e CredentialReportEntry) IsRoot() bool {
	return e.User == CredentialReportRootUser
}

// HasAccessKey reports whether the user holds at least one active access key, which is
// what makes a user a static-credential identity rather than a console-only one.
func (e CredentialReportEntry) HasAccessKey() bool {
	return e.AccessKey1Active || e.AccessKey2Active
}

// LastAccessKeyUse returns the most recent use of either access key, or nil when
// neither has ever been used.
//
// Both keys are considered whatever their active flag says: a key deactivated last week
// is evidence the user was in use last week, and treating it as never-used would score
// the user as dormant on the strength of the deactivation.
func (e CredentialReportEntry) LastAccessKeyUse() *time.Time {
	return laterOf(e.AccessKey1LastUsed, e.AccessKey2LastUsed)
}

// LastCredentialUse returns the most recent use of any credential the report covers:
// either access key, or the console password.
func (e CredentialReportEntry) LastCredentialUse() *time.Time {
	return laterOf(e.LastAccessKeyUse(), e.PasswordLastUsed)
}

func laterOf(a, b *time.Time) *time.Time {
	switch {
	case a == nil:
		return b
	case b == nil:
		return a
	case b.After(*a):
		return b
	default:
		return a
	}
}

// GetCredentialReport returns the account's credential report, generating it first if
// AWS does not have a current one.
//
// Two calls per account in the common case: GenerateCredentialReport reports COMPLETE
// when a report from the last four hours exists, and GetCredentialReport then returns
// it. When AWS is still building one this polls, bounded by
// credentialReportPollAttempts.
//
// It is a Get* method so the generated cached wrapper covers it.
func (r *IamRepository) GetCredentialReport() ([]CredentialReportEntry, error) {
	start := time.Now()

	if err := r.generateCredentialReport(); err != nil {
		return nil, err
	}

	var content []byte
	for attempt := 0; attempt < credentialReportPollAttempts; attempt++ {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.With(r.promLabels("GetCredentialReport", cfg.ResourceTypeUser)).Inc()
		}

		resp, err := r.iamClient().GetCredentialReport(r.ctx, &awsiam.GetCredentialReportInput{})
		if err == nil {
			content = resp.Content
			break
		}

		// Not ready and not present both mean "ask again shortly"; anything else is a
		// real failure and retrying it only delays the report of it.
		var notReady *types.CredentialReportNotReadyException
		var notPresent *types.CredentialReportNotPresentException
		if !stderrors.As(err, &notReady) && !stderrors.As(err, &notPresent) {
			if metrics.AwsMetricsEnabled {
				metrics.AwsApiRequestErrors.With(r.promLabels("GetCredentialReport", cfg.ResourceTypeUser)).Inc()
			}

			return nil, errors.New(err)
		}

		log.Debug().
			Int("attempt", attempt+1).
			Msg("[IamRepository.GetCredentialReport] report not ready, waiting")

		select {
		case <-r.ctx.Done():
			return nil, errors.New(r.ctx.Err())
		case <-time.After(credentialReportPollInterval):
		}
	}

	if content == nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("GetCredentialReport", cfg.ResourceTypeUser)).Inc()
		}

		return nil, errors.Errorf(
			"credential report not ready after %d attempts",
			credentialReportPollAttempts,
		)
	}

	entries, err := ParseCredentialReport(content)
	if err != nil {
		return nil, err
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("GetCredentialReport", cfg.ResourceTypeUser)).
			Add(float64(len(entries)))

		metrics.AwsRepoCallDuration.
			With(r.promLabels("GetCredentialReport", cfg.ResourceTypeUser)).
			Observe(time.Since(start).Seconds())
	}

	return entries, nil
}

// generateCredentialReport asks AWS to build a report, which is a no-op returning
// COMPLETE when a recent one already exists.
//
// ReportGenerationLimitExceeded is not an error worth failing on: it means generation
// was requested too often, and a report from the last four hours is therefore already
// available — which is what the caller wanted.
func (r *IamRepository) generateCredentialReport() error {
	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("GenerateCredentialReport", cfg.ResourceTypeUser)).Inc()
	}

	resp, err := r.iamClient().GenerateCredentialReport(r.ctx, &awsiam.GenerateCredentialReportInput{})
	if err != nil {
		var limitExceeded *types.ReportGenerationLimitExceededException
		if stderrors.As(err, &limitExceeded) {
			log.Debug().Msg("[IamRepository.generateCredentialReport] generation rate-limited, using the existing report")
			return nil
		}

		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("GenerateCredentialReport", cfg.ResourceTypeUser)).Inc()
		}

		return errors.New(err)
	}

	log.Debug().
		Str("state", string(resp.State)).
		Msg("[IamRepository.generateCredentialReport] report state")

	return nil
}

// ParseCredentialReport parses the report CSV into entries.
//
// Columns are read by header name rather than position. AWS has added columns to this
// report over time and appends them in the middle of the row, so an index-based parser
// silently starts reading the wrong field; a name-based one just ignores what it does
// not know. An unknown column is skipped, and a missing one leaves its field at the zero
// value rather than failing the whole report.
func ParseCredentialReport(content []byte) ([]CredentialReportEntry, error) {
	reader := csv.NewReader(strings.NewReader(string(content)))

	header, err := reader.Read()
	if err != nil {
		if stderrors.Is(err, io.EOF) {
			return nil, errors.New("credential report is empty")
		}

		return nil, errors.New(err)
	}

	column := make(map[string]int, len(header))
	for i, name := range header {
		column[strings.TrimSpace(name)] = i
	}

	var entries []CredentialReportEntry
	for {
		row, err := reader.Read()
		if stderrors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return entries, errors.New(err)
		}

		field := func(name string) string {
			i, ok := column[name]
			if !ok || i >= len(row) {
				return ""
			}

			return strings.TrimSpace(row[i])
		}

		entries = append(entries, CredentialReportEntry{
			User:                  field("user"),
			Arn:                   field("arn"),
			UserCreationTime:      parseReportTime(field("user_creation_time")),
			PasswordEnabled:       field("password_enabled") == "true",
			PasswordLastUsed:      parseReportTime(field("password_last_used")),
			PasswordLastChanged:   parseReportTime(field("password_last_changed")),
			MfaActive:             field("mfa_active") == "true",
			AccessKey1Active:      field("access_key_1_active") == "true",
			AccessKey1LastRotated: parseReportTime(field("access_key_1_last_rotated")),
			AccessKey1LastUsed:    parseReportTime(field("access_key_1_last_used_date")),
			AccessKey2Active:      field("access_key_2_active") == "true",
			AccessKey2LastRotated: parseReportTime(field("access_key_2_last_rotated")),
			AccessKey2LastUsed:    parseReportTime(field("access_key_2_last_used_date")),
		})
	}

	return entries, nil
}

// parseReportTime parses one report timestamp, returning nil for every way the report
// spells "there is no date here".
//
// N/A, no_information and not_supported are all real values in this CSV, and none of
// them is a date. Returning the zero time for them would read as January 1st year one —
// maximally dormant — which is the opposite of the truth for not_supported.
func parseReportTime(value string) *time.Time {
	switch value {
	case "", "N/A", "not_supported", "no_information":
		return nil
	}

	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		log.Debug().Str("value", value).Err(err).Msg("[iam.parseReportTime] unparseable report timestamp")
		return nil
	}

	return &parsed
}
