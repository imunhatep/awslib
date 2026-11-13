package cloudwatchlogs

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awscloudwatchlogs "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	"github.com/imunhatep/awslib/service"
	ccfg "github.com/imunhatep/awslib/service/cfg"
	"github.com/rs/zerolog/log"
)

const (
	// DefaultQueryPollMin and DefaultQueryPollMax bound the delay between two GetQueryResults polls
	// while a query runs. The delay grows by half each poll: a quick query is seen finishing within
	// a second, and a long one costs one request every few seconds.
	DefaultQueryPollMin = 500 * time.Millisecond
	DefaultQueryPollMax = 5 * time.Second

	// QueryRowsBufferSize is how many rows ListQueryResultsByInputAsync buffers for a query that
	// sets no Limit. AWS returns no more than 10,000 rows for such a query, so the buffer holds the
	// whole result and the producer never waits on the reader; a query with a Limit gets a buffer of
	// that size for the same reason.
	QueryRowsBufferSize = 10000

	// stopQueryTimeout bounds the StopQuery sent for an abandoned query, which runs on a context
	// detached from the repository's: the usual reason to abandon one is that context ending.
	stopQueryTimeout = 10 * time.Second
)

// insightsAPI is the part of the SDK client Logs Insights needs; tests substitute it.
type insightsAPI interface {
	StartQuery(context.Context, *awscloudwatchlogs.StartQueryInput, ...func(*awscloudwatchlogs.Options)) (*awscloudwatchlogs.StartQueryOutput, error)
	GetQueryResults(context.Context, *awscloudwatchlogs.GetQueryResultsInput, ...func(*awscloudwatchlogs.Options)) (*awscloudwatchlogs.GetQueryResultsOutput, error)
	StopQuery(context.Context, *awscloudwatchlogs.StopQueryInput, ...func(*awscloudwatchlogs.Options)) (*awscloudwatchlogs.StopQueryOutput, error)
}

func (r *CloudWatchLogsRepository) insightsAPIClient() insightsAPI {
	if r.insightsClient != nil {
		return r.insightsClient
	}

	return r.cloudwatchlogsClient()
}

// WithPollInterval bounds the delay between two status polls while waiting on a Logs Insights
// query; passing the same value for both polls at a fixed rate. Zero keeps the default for that
// bound. It returns the repository for chaining.
func (r *CloudWatchLogsRepository) WithPollInterval(min, max time.Duration) *CloudWatchLogsRepository {
	r.pollMin = min
	r.pollMax = max

	return r
}

func (r *CloudWatchLogsRepository) pollBounds() (time.Duration, time.Duration) {
	min, max := r.pollMin, r.pollMax
	if min <= 0 {
		min = DefaultQueryPollMin
	}
	if max <= 0 {
		max = DefaultQueryPollMax
	}
	if max < min {
		max = min
	}

	return min, max
}

// StartQuery starts a Logs Insights query and returns its ID. The query runs, and is billed, until
// it completes or StopQuery ends it; WaitForQuery collects it. ListQueryResultsByInput does all
// three.
func (r *CloudWatchLogsRepository) StartQuery(input *awscloudwatchlogs.StartQueryInput) (string, error) {
	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("StartQuery", ccfg.ResourceTypeCloudWatchLogsQuery)).Inc()
	}

	out, err := r.insightsAPIClient().StartQuery(r.ctx, input)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("StartQuery", ccfg.ResourceTypeCloudWatchLogsQuery)).Inc()
		}

		return "", errors.New(err)
	}

	return aws.ToString(out.QueryId), nil
}

// StopQuery stops a running query. AWS answers a query that has already finished with an error.
func (r *CloudWatchLogsRepository) StopQuery(queryID string) error {
	return r.stopQuery(r.ctx, queryID)
}

func (r *CloudWatchLogsRepository) stopQuery(ctx context.Context, queryID string) error {
	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("StopQuery", ccfg.ResourceTypeCloudWatchLogsQuery)).Inc()
	}

	if _, err := r.insightsAPIClient().StopQuery(ctx, &awscloudwatchlogs.StopQueryInput{QueryId: aws.String(queryID)}); err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("StopQuery", ccfg.ResourceTypeCloudWatchLogsQuery)).Inc()
		}

		return errors.New(err)
	}

	return nil
}

// abandonQuery stops a query this repository started and will not read: it would otherwise keep
// scanning, and billing, for up to an hour. Best effort, on a context of its own.
func (r *CloudWatchLogsRepository) abandonQuery(queryID string) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.ctx), stopQueryTimeout)
	defer cancel()

	if err := r.stopQuery(ctx, queryID); err != nil {
		log.Debug().Err(err).Str("queryID", queryID).Msg("[CloudWatchLogsRepository.abandonQuery] failed to stop query")
	}
}

// WaitForQuery polls a query until it ends and returns all of its rows. A query that ends in any
// status but Complete returns a *QueryFailedError. Unlike ListQueryResultsByInput it does not stop
// the query when it gives up: the caller started it and decides.
func (r *CloudWatchLogsRepository) WaitForQuery(queryID string) (QueryResults, error) {
	res := r.newQueryResults(queryID, "", 0)

	first, err := r.awaitQuery(&res)
	if err != nil {
		return res, err
	}

	err = r.readQueryPages(&res, first, func(rows []QueryRow) error {
		res.Rows = append(res.Rows, rows...)
		return nil
	})

	return res, err
}

// ListQueryResultsByInput runs a Logs Insights query to the end: start, wait, read every page. A
// query abandoned on the way, because the context ended or a poll failed, is stopped. Only a
// Complete query returns a nil error, so the cached repository never stores a partial answer.
//
// Cached, the result is only as fresh as the time range is closed: a query whose range ends now
// keeps answering with the rows of its first run.
func (r *CloudWatchLogsRepository) ListQueryResultsByInput(input *awscloudwatchlogs.StartQueryInput) (QueryResults, error) {
	rows := make([]QueryRow, 0)
	res, err := r.runQuery(input, func(page []QueryRow) error {
		rows = append(rows, page...)
		return nil
	})
	res.Rows = rows

	return res, err
}

// ListQueryResultsByInputAsync runs the query like ListQueryResultsByInput and streams its rows,
// page by page as they are read. Both channels are closed when the query is read, fails, or the
// repository context is done. The first error ends the stream and is the only value errChan ever
// carries.
//
// Rows arrive only once the query is Complete. Logs Insights does return rows while a query runs,
// but those are a preview, not a prefix of the answer: a sort or limit can still evict them, and a
// stats row's value keeps changing. The buffer is as large as the query's limit, so the producer
// never blocks on the reader.
func (r *CloudWatchLogsRepository) ListQueryResultsByInputAsync(input *awscloudwatchlogs.StartQueryInput) (<-chan QueryRow, <-chan *errors.Error) {
	size := QueryRowsBufferSize
	if limit := int(aws.ToInt32(input.Limit)); limit > 0 {
		size = limit
	}

	source := make(chan QueryRow, size)
	errChan := make(chan *errors.Error, 1)

	go func() {
		defer close(source)
		defer close(errChan)

		_, err := r.runQuery(input, func(rows []QueryRow) error {
			for _, row := range rows {
				if err := service.WriteToChan(source, row); err != nil {
					// more rows than the limit the buffer was sized for; end with an error rather
					// than hand back a result that only looks complete
					return errors.Errorf("reader fell %d rows behind: %w", cap(source), err)
				}
			}

			return nil
		})

		if err != nil {
			errChan <- errors.New(err)
		}
	}()

	return source, errChan
}

func (r *CloudWatchLogsRepository) ListQueryResultsByQuery(query *QueryMiddleware) (QueryResults, error) {
	if errs, ok := query.Errors(); !ok {
		return r.newQueryResults("", "", 0), errors.New(errs[0])
	}

	return r.ListQueryResultsByInput(query.Get())
}

func (r *CloudWatchLogsRepository) ListQueryResultsByQueryAsync(query *QueryMiddleware) (<-chan QueryRow, <-chan *errors.Error) {
	if errs, ok := query.Errors(); !ok {
		errChan := make(chan *errors.Error, 1)
		errChan <- errors.New(errs[0])
		close(errChan)

		// closed rather than nil: ranging over a nil channel blocks forever
		source := make(chan QueryRow)
		close(source)

		return source, errChan
	}

	return r.ListQueryResultsByInputAsync(query.Get())
}

func (r *CloudWatchLogsRepository) newQueryResults(queryID string, language types.QueryLanguage, limit int32) QueryResults {
	return QueryResults{
		AccountID:     r.client.GetAccountID(),
		Region:        r.client.GetRegion(),
		QueryID:       queryID,
		QueryLanguage: language,
		Limit:         limit,
		Rows:          []QueryRow{},
	}
}

func (r *CloudWatchLogsRepository) runQuery(input *awscloudwatchlogs.StartQueryInput, emit func([]QueryRow) error) (QueryResults, error) {
	start := time.Now()

	// a query started without a language runs as Logs Insights QL
	language := input.QueryLanguage
	if language == "" {
		language = types.QueryLanguageCwli
	}

	res := r.newQueryResults("", language, aws.ToInt32(input.Limit))

	queryID, err := r.StartQuery(input)
	if err != nil {
		return res, err
	}

	res.QueryID = queryID

	log.Debug().
		Str("queryID", queryID).
		Str("query", aws.ToString(input.QueryString)).
		Msg("[CloudWatchLogsRepository.runQuery] query started")

	first, err := r.awaitQuery(&res)
	if err != nil {
		var failed *QueryFailedError
		if !errors.As(err, &failed) {
			r.abandonQuery(queryID)
		}

		return res, err
	}

	rowCount := 0
	err = r.readQueryPages(&res, first, func(rows []QueryRow) error {
		rowCount += len(rows)
		return emit(rows)
	})

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("GetQueryResults", ccfg.ResourceTypeCloudWatchLogsQuery)).
			Add(float64(rowCount))

		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListQueryResultsByInput", ccfg.ResourceTypeCloudWatchLogsQuery)).
			Observe(time.Since(start).Seconds())
	}

	return res, err
}

// awaitQuery polls res.QueryID until the query leaves Scheduled and Running. On Complete it returns
// the poll's response when that already is the first page of rows, else nil.
//
// Polls of a Logs Insights QL query ask for a single row: a running query returns its preview rows
// on every poll, up to 10,000 of them, all thrown away. SQL and PPL results cannot be paged, so
// their polls ask for everything, as do the polls of a query whose language is not known yet.
func (r *CloudWatchLogsRepository) awaitQuery(res *QueryResults) (*awscloudwatchlogs.GetQueryResultsOutput, error) {
	delay, maxDelay := r.pollBounds()

	for {
		trimmed := res.QueryLanguage == types.QueryLanguageCwli

		input := &awscloudwatchlogs.GetQueryResultsInput{QueryId: aws.String(res.QueryID)}
		if trimmed {
			input.MaxItems = aws.Int32(1)
		}

		out, err := r.getQueryResults(input)
		if err != nil {
			return nil, err
		}

		if out.QueryLanguage != "" {
			res.QueryLanguage = out.QueryLanguage
		}

		res.Status = out.Status

		switch out.Status {
		case types.QueryStatusComplete:
			if trimmed {
				return nil, nil
			}

			return out, nil

		case types.QueryStatusScheduled, types.QueryStatusRunning:
			log.Trace().
				Str("queryID", res.QueryID).
				Str("status", string(out.Status)).
				Dur("next", delay).
				Msg("[CloudWatchLogsRepository.awaitQuery] query not finished")

		default:
			// Failed, Cancelled, Timeout, and Unknown, which is treated as final rather than polled
			// for up to the hour a query may run
			return nil, errors.New(&QueryFailedError{QueryID: res.QueryID, Status: out.Status})
		}

		if err := r.sleep(delay); err != nil {
			return nil, err
		}

		delay = min(delay*3/2, maxDelay)
	}
}

// readQueryPages hands every page of a Complete query to emit, starting with first when the poll
// already read it. Statistics are taken from the last page.
func (r *CloudWatchLogsRepository) readQueryPages(res *QueryResults, first *awscloudwatchlogs.GetQueryResultsOutput, emit func([]QueryRow) error) error {
	out := first
	var token *string

	for {
		if out == nil {
			var err error
			out, err = r.getQueryResults(&awscloudwatchlogs.GetQueryResultsInput{
				QueryId:   aws.String(res.QueryID),
				NextToken: token,
			})
			if err != nil {
				return err
			}
		}

		if out.Statistics != nil {
			res.Statistics = *out.Statistics
		}

		rows := make([]QueryRow, 0, len(out.Results))
		for _, fields := range out.Results {
			rows = append(rows, newQueryRow(r.client, fields))
		}

		if err := emit(rows); err != nil {
			return err
		}

		next := out.NextToken
		if aws.ToString(next) == "" {
			return nil
		}

		if token != nil && *next == *token {
			return errors.Errorf("GetQueryResults returned the same nextToken twice for query %s", res.QueryID)
		}

		token, out = next, nil
	}
}

func (r *CloudWatchLogsRepository) getQueryResults(input *awscloudwatchlogs.GetQueryResultsInput) (*awscloudwatchlogs.GetQueryResultsOutput, error) {
	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("GetQueryResults", ccfg.ResourceTypeCloudWatchLogsQuery)).Inc()
	}

	out, err := r.insightsAPIClient().GetQueryResults(r.ctx, input)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("GetQueryResults", ccfg.ResourceTypeCloudWatchLogsQuery)).Inc()
		}

		return nil, errors.New(err)
	}

	return out, nil
}

func (r *CloudWatchLogsRepository) sleep(d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()

	select {
	case <-t.C:
		return nil
	case <-r.ctx.Done():
		return errors.New(r.ctx.Err())
	}
}
