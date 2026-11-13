package cloudwatchlogs

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	ptypes "github.com/imunhatep/awslib/provider/types"
)

const (
	// QueryFieldPtr is the hidden field Logs Insights adds to every row backed by one log event;
	// GetLogRecord reads the full event with it. Rows of an aggregation (stats) carry none.
	QueryFieldPtr       = "@ptr"
	QueryFieldTimestamp = "@timestamp"
	QueryFieldMessage   = "@message"
	QueryFieldLogStream = "@logStream"
	QueryFieldLog       = "@log"

	// queryTimestampLayout is how Logs Insights renders @timestamp, always in UTC.
	queryTimestampLayout = "2006-01-02 15:04:05.000"
)

// QueryField is one field / value pair of a Logs Insights result row.
type QueryField struct {
	Name  string
	Value string
}

// QueryRow is one row of a Logs Insights result, its fields in the order AWS returned them. It
// carries the account and region it was read from, so rows of a fan-out over many clients can
// share one channel.
type QueryRow struct {
	AccountID ptypes.AwsAccountID
	Region    ptypes.AwsRegion
	Fields    []QueryField
}

func newQueryRow(client AwsClient, fields []types.ResultField) QueryRow {
	row := QueryRow{
		AccountID: client.GetAccountID(),
		Region:    client.GetRegion(),
		Fields:    make([]QueryField, 0, len(fields)),
	}

	for _, f := range fields {
		row.Fields = append(row.Fields, QueryField{Name: aws.ToString(f.Field), Value: aws.ToString(f.Value)})
	}

	return row
}

// Lookup returns the value of field name and whether the row has it. Logs Insights leaves out a
// field the event does not have, so absent and empty are different answers.
func (r QueryRow) Lookup(name string) (string, bool) {
	for _, f := range r.Fields {
		if f.Name == name {
			return f.Value, true
		}
	}

	return "", false
}

func (r QueryRow) Get(name string) string {
	v, _ := r.Lookup(name)
	return v
}

// Map returns the row as a new map from field name to value; the field order is lost.
func (r QueryRow) Map() map[string]string {
	m := make(map[string]string, len(r.Fields))
	for _, f := range r.Fields {
		m[f.Name] = f.Value
	}

	return m
}

func (r QueryRow) GetPtr() string {
	return r.Get(QueryFieldPtr)
}

func (r QueryRow) GetMessage() string {
	return r.Get(QueryFieldMessage)
}

// GetTimestamp parses @timestamp. The second value is false when the row has none (an aggregation
// row, or a query whose fields command left it out) or it does not parse.
func (r QueryRow) GetTimestamp() (time.Time, bool) {
	v, ok := r.Lookup(QueryFieldTimestamp)
	if !ok {
		return time.Time{}, false
	}

	t, err := time.ParseInLocation(queryTimestampLayout, v, time.UTC)
	if err != nil {
		return time.Time{}, false
	}

	return t, true
}

// QueryResults is a finished Logs Insights query: its rows and what AWS reports about the run.
type QueryResults struct {
	AccountID     ptypes.AwsAccountID
	Region        ptypes.AwsRegion
	QueryID       string
	QueryLanguage types.QueryLanguage
	Status        types.QueryStatus

	// Limit is the row limit the query was started with, zero when it was left to AWS's default or
	// is not known (WaitForQuery on a query started elsewhere).
	Limit int32

	Rows []QueryRow

	// Statistics describe the whole run. BytesScanned is what the query is billed on.
	Statistics types.QueryStatistics
}

// LimitReached reports that the query returned as many rows as its limit allowed, so more rows
// may have matched than are here. It can only tell when the limit is known.
func (q QueryResults) LimitReached() bool {
	return q.Limit > 0 && len(q.Rows) >= int(q.Limit)
}

// QueryFailedError is returned when a query ended in a status other than Complete: Failed,
// Cancelled, Timeout (AWS stops a query after 60 minutes) or Unknown. Reach it with errors.As:
//
//	var failed *cloudwatchlogs.QueryFailedError
//	if errors.As(err, &failed) && failed.Status == types.QueryStatusTimeout { ... }
type QueryFailedError struct {
	QueryID string
	Status  types.QueryStatus
}

func (e *QueryFailedError) Error() string {
	return fmt.Sprintf("cloudwatchlogs: insights query %s ended %s", e.QueryID, e.Status)
}
