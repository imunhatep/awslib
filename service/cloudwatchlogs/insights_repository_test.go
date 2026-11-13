package cloudwatchlogs

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awscloudwatchlogs "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/go-errors/errors"
	v3 "github.com/imunhatep/awslib/provider/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeInsights runs one query: runningPolls polls answer Running with preview rows, then the query
// ends in final. A Complete query has rows rows, served pageSize at a time.
type fakeInsights struct {
	runningPolls int
	final        types.QueryStatus
	language     types.QueryLanguage // reported by GetQueryResults; empty reports none
	rows         int
	pageSize     int
	pollErr      error
	repeatToken  bool

	mu        sync.Mutex
	polls     int
	maxItems  []int32 // MaxItems of each GetQueryResults request, 0 when unset
	stopCalls int
}

func (f *fakeInsights) StartQuery(_ context.Context, in *awscloudwatchlogs.StartQueryInput, _ ...func(*awscloudwatchlogs.Options)) (*awscloudwatchlogs.StartQueryOutput, error) {
	return &awscloudwatchlogs.StartQueryOutput{QueryId: aws.String("q-1")}, nil
}

func (f *fakeInsights) StopQuery(_ context.Context, _ *awscloudwatchlogs.StopQueryInput, _ ...func(*awscloudwatchlogs.Options)) (*awscloudwatchlogs.StopQueryOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stopCalls++

	return &awscloudwatchlogs.StopQueryOutput{Success: true}, nil
}

func (f *fakeInsights) GetQueryResults(_ context.Context, in *awscloudwatchlogs.GetQueryResultsInput, _ ...func(*awscloudwatchlogs.Options)) (*awscloudwatchlogs.GetQueryResultsOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.maxItems = append(f.maxItems, aws.ToInt32(in.MaxItems))

	if in.NextToken == nil {
		f.polls++
		if f.pollErr != nil {
			return nil, f.pollErr
		}

		if f.polls <= f.runningPolls {
			return &awscloudwatchlogs.GetQueryResultsOutput{
				Status:        types.QueryStatusRunning,
				QueryLanguage: f.language,
				Results:       f.page(0, f.limit(in, f.rows)),
			}, nil
		}
	}

	if f.final != types.QueryStatusComplete {
		return &awscloudwatchlogs.GetQueryResultsOutput{Status: f.final, QueryLanguage: f.language}, nil
	}

	offset := 0
	if in.NextToken != nil {
		offset, _ = strconv.Atoi(*in.NextToken)
	}

	end := min(offset+f.limit(in, f.pageSize), f.rows)
	out := &awscloudwatchlogs.GetQueryResultsOutput{
		Status:        types.QueryStatusComplete,
		QueryLanguage: f.language,
		Results:       f.page(offset, end),
		Statistics:    &types.QueryStatistics{RecordsMatched: float64(f.rows), BytesScanned: 1024},
	}

	if end < f.rows {
		out.NextToken = aws.String(strconv.Itoa(end))
		if f.repeatToken && in.NextToken != nil {
			out.NextToken = in.NextToken
		}
	}

	return out, nil
}

func (f *fakeInsights) limit(in *awscloudwatchlogs.GetQueryResultsInput, n int) int {
	if in.MaxItems != nil {
		return min(n, int(*in.MaxItems))
	}

	return n
}

func (f *fakeInsights) page(from, to int) [][]types.ResultField {
	var rows [][]types.ResultField
	for i := from; i < to; i++ {
		rows = append(rows, []types.ResultField{
			{Field: aws.String("@timestamp"), Value: aws.String("2026-11-13 10:00:00.000")},
			{Field: aws.String("@message"), Value: aws.String(fmt.Sprintf("row-%d", i))},
			{Field: aws.String("@ptr"), Value: aws.String(fmt.Sprintf("ptr-%d", i))},
		})
	}

	return rows
}

func (f *fakeInsights) stops() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.stopCalls
}

func newTestInsightsRepository(ctx context.Context, api insightsAPI) *CloudWatchLogsRepository {
	repo := &CloudWatchLogsRepository{ctx: ctx, client: &v3.Client{}, insightsClient: api}
	return repo.WithPollInterval(time.Millisecond, time.Millisecond)
}

func testQuery() *awscloudwatchlogs.StartQueryInput {
	return NewQueryMiddleware("fields @timestamp, @message").
		WithLogGroupNames("/aws/lambda/test").
		WithStartTime(time.Now().Add(-time.Hour)).
		WithEndTime(time.Now()).
		Get()
}

func TestListQueryResultsReadsEveryPage(t *testing.T) {
	api := &fakeInsights{runningPolls: 2, final: types.QueryStatusComplete, rows: 25, pageSize: 10}

	res, err := newTestInsightsRepository(context.Background(), api).ListQueryResultsByInput(testQuery())
	require.NoError(t, err)

	require.Len(t, res.Rows, 25)
	assert.Equal(t, "row-0", res.Rows[0].GetMessage())
	assert.Equal(t, "row-24", res.Rows[24].GetMessage())
	assert.Equal(t, types.QueryStatusComplete, res.Status)
	assert.Equal(t, "q-1", res.QueryID)
	assert.Equal(t, float64(1024), res.Statistics.BytesScanned)
	assert.Zero(t, api.stops(), "a completed query must not be stopped")

	// three polls asking for one row each, then the three pages in full
	assert.Equal(t, []int32{1, 1, 1, 0, 0, 0}, api.maxItems)
}

func TestListQueryResultsFailedStatus(t *testing.T) {
	for _, status := range []types.QueryStatus{
		types.QueryStatusFailed, types.QueryStatusCancelled, types.QueryStatusTimeout, types.QueryStatusUnknown,
	} {
		t.Run(string(status), func(t *testing.T) {
			api := &fakeInsights{runningPolls: 1, final: status}

			res, err := newTestInsightsRepository(context.Background(), api).ListQueryResultsByInput(testQuery())

			var failed *QueryFailedError
			require.True(t, errors.As(err, &failed), "error %v is not a *QueryFailedError", err)
			assert.Equal(t, status, failed.Status)
			assert.Equal(t, status, res.Status)
			assert.Zero(t, api.stops(), "a query that already ended must not be stopped")
		})
	}
}

// A query nobody will read keeps scanning, and billing, for up to an hour unless it is stopped.
func TestListQueryResultsStopsAbandonedQuery(t *testing.T) {
	t.Run("context cancelled", func(t *testing.T) {
		api := &fakeInsights{runningPolls: 1 << 30, final: types.QueryStatusComplete}
		ctx, cancel := context.WithCancel(context.Background())
		time.AfterFunc(20*time.Millisecond, cancel)

		_, err := newTestInsightsRepository(ctx, api).ListQueryResultsByInput(testQuery())
		require.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, 1, api.stops())
	})

	t.Run("poll failed", func(t *testing.T) {
		api := &fakeInsights{pollErr: fmt.Errorf("throttled"), final: types.QueryStatusComplete}

		_, err := newTestInsightsRepository(context.Background(), api).ListQueryResultsByInput(testQuery())
		require.Error(t, err)
		assert.Equal(t, 1, api.stops())
	})
}

func TestListQueryResultsRepeatedTokenFails(t *testing.T) {
	api := &fakeInsights{final: types.QueryStatusComplete, rows: 30, pageSize: 10, repeatToken: true}

	done := make(chan error, 1)
	go func() {
		_, err := newTestInsightsRepository(context.Background(), api).ListQueryResultsByInput(testQuery())
		done <- err
	}()

	select {
	case err := <-done:
		require.Error(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("a repeated nextToken looped forever")
	}
}

// WaitForQuery does not know the language of a query started elsewhere, and SQL and PPL results
// cannot be paged, so it must not trim its polls until a response says Logs Insights QL.
func TestWaitForQueryTrimsPollsOnlyForInsightsQL(t *testing.T) {
	t.Run("CWLI", func(t *testing.T) {
		api := &fakeInsights{runningPolls: 2, final: types.QueryStatusComplete, language: types.QueryLanguageCwli, rows: 5, pageSize: 10}

		res, err := newTestInsightsRepository(context.Background(), api).WaitForQuery("q-1")
		require.NoError(t, err)
		assert.Len(t, res.Rows, 5)
		assert.Equal(t, []int32{0, 1, 1, 0}, api.maxItems)
	})

	t.Run("SQL", func(t *testing.T) {
		api := &fakeInsights{runningPolls: 2, final: types.QueryStatusComplete, language: types.QueryLanguageSql, rows: 5, pageSize: 10}

		res, err := newTestInsightsRepository(context.Background(), api).WaitForQuery("q-1")
		require.NoError(t, err)
		assert.Len(t, res.Rows, 5)
		// the poll that saw Complete already carried the rows, so nothing is read twice
		assert.Equal(t, []int32{0, 0, 0}, api.maxItems)
	})
}

func TestListQueryResultsAsyncDeliversEveryRowToSlowReader(t *testing.T) {
	api := &fakeInsights{final: types.QueryStatusComplete, rows: 250, pageSize: 100}
	query := testQuery()
	query.Limit = aws.Int32(250)

	source, errChan := newTestInsightsRepository(context.Background(), api).ListQueryResultsByInputAsync(query)

	seen := map[string]bool{}
	for row := range source {
		time.Sleep(10 * time.Microsecond)
		seen[row.GetPtr()] = true
	}

	err, ok := <-errChan
	require.False(t, ok, "unexpected error: %v", err)
	assert.Len(t, seen, 250)
}

func TestListQueryResultsByQueryAsyncInvalidQueryClosesRows(t *testing.T) {
	source, errChan := newTestInsightsRepository(context.Background(), &fakeInsights{}).
		ListQueryResultsByQueryAsync(NewQueryMiddleware(""))

	select {
	case _, ok := <-source:
		require.False(t, ok, "expected no rows for an invalid query")
	case <-time.After(time.Second):
		t.Fatal("row channel for an invalid query is not closed")
	}

	err, ok := <-errChan
	require.True(t, ok)
	require.Error(t, err)
}

func TestQueryMiddlewareValidation(t *testing.T) {
	start := time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)

	valid := func() *QueryMiddleware {
		return NewQueryMiddleware("fields @message").WithLogGroupNames("g").WithStartTime(start).WithEndTime(end)
	}

	_, ok := valid().Errors()
	assert.True(t, ok)

	cases := map[string]*QueryMiddleware{
		"empty query":    NewQueryMiddleware(" ").WithLogGroupNames("g").WithStartTime(start).WithEndTime(end),
		"no end time":    NewQueryMiddleware("fields @message").WithLogGroupNames("g").WithStartTime(start),
		"inverted range": NewQueryMiddleware("fields @message").WithLogGroupNames("g").WithStartTime(end).WithEndTime(start),
		"no log groups":  NewQueryMiddleware("fields @message").WithStartTime(start).WithEndTime(end),
		"limit zero":     valid().WithLimit(0),
		"limit too high": valid().WithLimit(QueryMaxLimit + 1),
		"wildcard group": valid().WithLogGroupNames("arn:aws:logs:eu-west-1:123:log-group:g:*"),
		"too many groups": valid().WithLogGroupNames(func() []string {
			ids := make([]string, QueryMaxLogGroups)
			for i := range ids {
				ids[i] = fmt.Sprintf("g%d", i)
			}
			return ids
		}()...),
		"mixed selectors": NewQueryMiddleware("fields @message").
			WithHandler(func(q *awscloudwatchlogs.StartQueryInput) (*awscloudwatchlogs.StartQueryInput, error) {
				q.LogGroupNames = []string{"a"}
				return q, nil
			}).
			WithLogGroupNames("b").WithStartTime(start).WithEndTime(end),
	}

	for name, query := range cases {
		t.Run(name, func(t *testing.T) {
			_, ok := query.Errors()
			assert.False(t, ok)
		})
	}

	t.Run("SQL names its log groups in the query", func(t *testing.T) {
		_, ok := NewQueryMiddleware("SELECT * FROM `logGroups(logGroupIdentifier: ['g'])`").
			WithQueryLanguage(types.QueryLanguageSql).WithStartTime(start).WithEndTime(end).Errors()
		assert.True(t, ok)
	})
}

// StartQuery takes whole seconds and includes the end, so a fractional end is rounded up.
func TestQueryTimeRangeSeconds(t *testing.T) {
	start := time.Date(2026, 11, 13, 10, 0, 0, 900_000_000, time.UTC)
	q := NewQueryMiddleware("fields @message").WithStartTime(start).WithEndTime(start.Add(time.Minute)).Get()

	assert.Equal(t, start.Unix(), aws.ToInt64(q.StartTime))
	assert.Equal(t, start.Unix()+61, aws.ToInt64(q.EndTime))
}

func TestWithLogGroupsUsesArnWithoutWildcard(t *testing.T) {
	q := NewQueryMiddleware("fields @message").WithLogGroups(
		LogGroup{LogGroup: types.LogGroup{LogGroupArn: aws.String("arn:aws:logs:eu-west-1:123:log-group:a")}},
		LogGroup{LogGroup: types.LogGroup{Arn: aws.String("arn:aws:logs:eu-west-1:123:log-group:b:*")}},
		LogGroup{LogGroup: types.LogGroup{LogGroupName: aws.String("c")}},
	)

	_, _ = q.Errors()
	assert.Equal(t, []string{
		"arn:aws:logs:eu-west-1:123:log-group:a",
		"arn:aws:logs:eu-west-1:123:log-group:b",
		"c",
	}, q.Get().LogGroupIdentifiers)
}

func TestQueryRowAccessors(t *testing.T) {
	row := newQueryRow(&v3.Client{}, []types.ResultField{
		{Field: aws.String("@timestamp"), Value: aws.String("2026-11-13 10:00:01.250")},
		{Field: aws.String("level"), Value: aws.String("")},
	})

	ts, ok := row.GetTimestamp()
	require.True(t, ok)
	assert.Equal(t, time.Date(2026, 11, 13, 10, 0, 1, 250_000_000, time.UTC), ts)

	v, ok := row.Lookup("level")
	assert.True(t, ok, "an empty field is still present")
	assert.Empty(t, v)

	_, ok = row.Lookup("missing")
	assert.False(t, ok)

	assert.Equal(t, map[string]string{"@timestamp": "2026-11-13 10:00:01.250", "level": ""}, row.Map())
}

func TestQueryResultsLimitReached(t *testing.T) {
	assert.False(t, QueryResults{Rows: make([]QueryRow, 5)}.LimitReached(), "unknown limit")
	assert.False(t, QueryResults{Limit: 10, Rows: make([]QueryRow, 5)}.LimitReached())
	assert.True(t, QueryResults{Limit: 5, Rows: make([]QueryRow, 5)}.LimitReached())
}

// The cached repository stores QueryResults with gob.
func TestQueryResultsRoundTripThroughGob(t *testing.T) {
	in := QueryResults{
		QueryID:    "q-1",
		Status:     types.QueryStatusComplete,
		Limit:      10,
		Rows:       []QueryRow{{Fields: []QueryField{{Name: "@message", Value: "hello"}}}},
		Statistics: types.QueryStatistics{BytesScanned: 42, RecordsMatched: 1},
	}

	var buf bytes.Buffer
	require.NoError(t, gob.NewEncoder(&buf).Encode(in))

	var out QueryResults
	require.NoError(t, gob.NewDecoder(&buf).Decode(&out))
	assert.Equal(t, in.Rows, out.Rows)
	assert.Equal(t, in.Statistics.BytesScanned, out.Statistics.BytesScanned)
	assert.Equal(t, in.Status, out.Status)
}
