package cloudwatchlogs

import (
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awscloudwatchlogs "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/go-errors/errors"
	"github.com/rs/zerolog/log"
)

const (
	// QueryMaxLogGroups is how many log groups one StartQuery accepts.
	QueryMaxLogGroups = 50

	// QueryMaxLimit is the largest Limit StartQuery accepts. Results past 10,000 rows are read
	// over several GetQueryResults pages, which AWS supports for Logs Insights QL only.
	QueryMaxLimit = 100000
)

type QueryHandler func(*awscloudwatchlogs.StartQueryInput) (*awscloudwatchlogs.StartQueryInput, error)

// QueryMiddleware builds a Logs Insights StartQueryInput the way cloudtrail.LookupMiddleware builds
// a LookupEventsInput: every With* returns the builder, and errors are collected for Errors()
// rather than returned, so check it once before running the query.
type QueryMiddleware struct {
	query *awscloudwatchlogs.StartQueryInput
	errs  []error
}

func NewQueryMiddleware(queryString string) *QueryMiddleware {
	return &QueryMiddleware{
		query: &awscloudwatchlogs.StartQueryInput{QueryString: aws.String(queryString)},
		errs:  []error{},
	}
}

func (q *QueryMiddleware) Get() *awscloudwatchlogs.StartQueryInput {
	return q.query
}

// Errors returns the errors collected by the handlers plus those of a query that is not complete
// yet: no query string, no time range, an inverted range, or no log groups for a language that
// takes them as a parameter. The second value is true when there are none.
func (q *QueryMiddleware) Errors() ([]error, bool) {
	errs := append([]error{}, q.errs...)
	errs = append(errs, validateQuery(q.query)...)

	return errs, len(errs) == 0
}

// WithHandler applies f to the query. An error from f is collected for Errors() and leaves the
// query as it was.
func (q *QueryMiddleware) WithHandler(f QueryHandler) *QueryMiddleware {
	query, err := f(q.query)
	if err != nil {
		q.errs = append(q.errs, err)
		return q
	}

	if query != nil {
		q.query = query
	}

	return q
}

func (q *QueryMiddleware) WithStartTime(start time.Time) *QueryMiddleware {
	return q.WithHandler(QueryStartTimeHandler(start))
}

func (q *QueryMiddleware) WithEndTime(end time.Time) *QueryMiddleware {
	return q.WithHandler(QueryEndTimeHandler(end))
}

func (q *QueryMiddleware) WithLimit(limit int32) *QueryMiddleware {
	return q.WithHandler(QueryLimitHandler(limit))
}

func (q *QueryMiddleware) WithQueryLanguage(language types.QueryLanguage) *QueryMiddleware {
	return q.WithHandler(QueryLanguageHandler(language))
}

// WithLogGroupNames adds log groups by name or by ARN (without a trailing "*"). A log group in a
// source account has to be given by ARN when querying from a monitoring account.
func (q *QueryMiddleware) WithLogGroupNames(names ...string) *QueryMiddleware {
	return q.WithHandler(QueryLogGroupsHandler(names...))
}

// WithLogGroups adds log groups listed by ListLogGroupsAll/ListLogGroupsByInput, by ARN.
func (q *QueryMiddleware) WithLogGroups(groups ...LogGroup) *QueryMiddleware {
	ids := make([]string, 0, len(groups))
	for _, g := range groups {
		ids = append(ids, logGroupIdentifier(g))
	}

	return q.WithHandler(QueryLogGroupsHandler(ids...))
}

// logGroupIdentifier prefers LogGroupArn, which StartQuery accepts as is, over Arn, which carries
// the ":*" suffix StartQuery refuses, and falls back to the name.
func logGroupIdentifier(g LogGroup) string {
	if arn := aws.ToString(g.LogGroupArn); arn != "" {
		return arn
	}

	if arn := strings.TrimSuffix(aws.ToString(g.Arn), ":*"); arn != "" {
		return arn
	}

	return aws.ToString(g.LogGroupName)
}

func QueryStartTimeHandler(t time.Time) QueryHandler {
	return func(q *awscloudwatchlogs.StartQueryInput) (*awscloudwatchlogs.StartQueryInput, error) {
		log.Trace().Str("key", "StartTime").Time("time", t).Msg("insights: query")

		q.StartTime = aws.Int64(t.Unix())

		return q, nil
	}
}

// QueryEndTimeHandler sets the end of the range. StartQuery takes whole seconds and includes the
// end, so the end is rounded up: a range ending at 12:00:00.5 still covers events at 12:00:00.4.
func QueryEndTimeHandler(t time.Time) QueryHandler {
	return func(q *awscloudwatchlogs.StartQueryInput) (*awscloudwatchlogs.StartQueryInput, error) {
		log.Trace().Str("key", "EndTime").Time("time", t).Msg("insights: query")

		end := t.Unix()
		if t.Nanosecond() > 0 {
			end++
		}

		q.EndTime = aws.Int64(end)

		return q, nil
	}
}

func QueryLimitHandler(limit int32) QueryHandler {
	return func(q *awscloudwatchlogs.StartQueryInput) (*awscloudwatchlogs.StartQueryInput, error) {
		log.Trace().Str("key", "Limit").Int32("limit", limit).Msg("insights: query")

		if limit < 1 || limit > QueryMaxLimit {
			return q, errors.Errorf("query limit %d is outside 1..%d", limit, QueryMaxLimit)
		}

		q.Limit = aws.Int32(limit)

		return q, nil
	}
}

func QueryLanguageHandler(language types.QueryLanguage) QueryHandler {
	return func(q *awscloudwatchlogs.StartQueryInput) (*awscloudwatchlogs.StartQueryInput, error) {
		q.QueryLanguage = language

		return q, nil
	}
}

// QueryLogGroupsHandler adds log groups to LogGroupIdentifiers, which takes names and ARNs alike.
// StartQuery takes exactly one of LogGroupName, LogGroupNames and LogGroupIdentifiers, so a query
// that already sets either of the other two is refused rather than sent with both.
func QueryLogGroupsHandler(identifiers ...string) QueryHandler {
	return func(q *awscloudwatchlogs.StartQueryInput) (*awscloudwatchlogs.StartQueryInput, error) {
		log.Trace().Strs("logGroups", identifiers).Msg("insights: query")

		if q.LogGroupName != nil || len(q.LogGroupNames) > 0 {
			return q, errors.New("query already selects log groups by LogGroupName or LogGroupNames; StartQuery takes only one of them")
		}

		if n := len(q.LogGroupIdentifiers) + len(identifiers); n > QueryMaxLogGroups {
			return q, errors.Errorf("query would select %d log groups, StartQuery accepts at most %d", n, QueryMaxLogGroups)
		}

		for _, id := range identifiers {
			if id == "" || strings.HasSuffix(id, "*") {
				return q, errors.Errorf("invalid log group identifier %q: give a name or an ARN without a trailing *", id)
			}
		}

		q.LogGroupIdentifiers = append(q.LogGroupIdentifiers, identifiers...)

		return q, nil
	}
}

// validateQuery reports what StartQuery would refuse, so a builder mistake costs no API call.
func validateQuery(q *awscloudwatchlogs.StartQueryInput) []error {
	var errs []error

	if strings.TrimSpace(aws.ToString(q.QueryString)) == "" {
		errs = append(errs, errors.New("query string is empty"))
	}

	if q.StartTime == nil || q.EndTime == nil {
		errs = append(errs, errors.New("query needs both a start and an end time"))
	} else if *q.StartTime > *q.EndTime {
		errs = append(errs, errors.New("query start time is after its end time"))
	}

	// OpenSearch SQL names its log groups inside the query string
	selected := q.LogGroupName != nil || len(q.LogGroupNames) > 0 || len(q.LogGroupIdentifiers) > 0
	if !selected && q.QueryLanguage != types.QueryLanguageSql {
		errs = append(errs, errors.New("query selects no log groups"))
	}

	return errs
}
