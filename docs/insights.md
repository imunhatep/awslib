# CloudWatch Logs Insights

`service/cloudwatchlogs` runs Logs Insights queries next to its log group listing. The shape
follows the CloudTrail lookups in [observer.md](observer.md#related-cloudtrail-lookups): a
builder that collects its errors, a synchronous `List*` that the cached repository wraps, and an
`*Async` variant streaming rows through a channel.

- [Quick start](#quick-start)
- [Building a query](#building-a-query)
- [Running it](#running-it)
- [Reading the results](#reading-the-results)
- [How it behaves](#how-it-behaves)

## Quick start

```go
repo := cloudwatchlogs.NewCloudWatchLogsRepository(ctx, client)

query := cloudwatchlogs.NewQueryMiddleware(`fields @timestamp, @message
    | filter @message like /ERROR/
    | sort @timestamp desc`).
    WithLogGroupNames("/aws/lambda/checkout").
    WithStartTime(time.Now().Add(-time.Hour)).
    WithEndTime(time.Now()).
    WithLimit(500)

if errs, ok := query.Errors(); !ok {
    return errs[0]
}

res, err := repo.ListQueryResultsByQuery(query)
if err != nil {
    return err
}

for _, row := range res.Rows {
    ts, _ := row.GetTimestamp()
    fmt.Println(ts, row.GetMessage())
}

if res.LimitReached() {
    fmt.Println("more rows matched than the limit allowed")
}
```

## Building a query

`NewQueryMiddleware(queryString)` builds a `StartQueryInput`. Like the CloudTrail builder, every
`With*` returns the builder and errors are collected, so check `Errors()` once.

| Method | Sets |
|---|---|
| `WithStartTime(t)`, `WithEndTime(t)` | the range, both required. StartQuery takes whole seconds and includes the end, which is rounded up |
| `WithLogGroupNames(ids...)` | log groups by name or by ARN, at most 50 (`QueryMaxLogGroups`) |
| `WithLogGroups(groups...)` | log groups from `ListLogGroupsAll`, by their ARN |
| `WithLimit(n)` | the row limit, 1 to 100,000 (`QueryMaxLimit`) |
| `WithQueryLanguage(lang)` | `CWLI` (the default), `SQL` or `PPL` |
| `WithHandler(f)` | anything else, as a `QueryHandler` |

`Errors()` also reports what StartQuery would refuse: an empty query, a missing or inverted
range, and no log groups for a language that takes them as a parameter (OpenSearch SQL names
them inside the query string, so it is exempt).

Log groups always go into `LogGroupIdentifiers`, which accepts names and ARNs alike. A log group
in a source account must be given by ARN from the monitoring account. The ARN must not end in
`*`: `WithLogGroups` uses `LogGroupArn`, not the `Arn` that `DescribeLogGroups` returns with a
`:*` suffix.

## Running it

| Method | Does | Cached |
|---|---|---|
| `ListQueryResultsByQuery(q)` / `ListQueryResultsByInput(in)` | start, wait, read every page | yes |
| `ListQueryResultsByQueryAsync(q)` / `ListQueryResultsByInputAsync(in)` | the same, rows streamed to a channel | no |
| `StartQuery(in)` | start only, returns the query ID | no |
| `WaitForQuery(id)` | wait for a started query and read every page | no |
| `StopQuery(id)` | stop a running query | no |

`StartQuery` and `WaitForQuery` are the split for a fan-out: start one query per account and
region, which returns at once, then wait on each. `ListQueryResultsByInput` is the two together.

```go
repo := cloudwatchlogs.NewCloudWatchLogsRepository(ctx, client).
    WithPollInterval(time.Second, 10*time.Second)

rows, errChan := repo.ListQueryResultsByQueryAsync(query)
for row := range rows {
    fmt.Println(row.AccountID, row.Region, row.GetMessage())
}

if err, ok := <-errChan; ok {
    return err
}
```

Each `QueryRow` carries its account and region, so rows from many clients can share a channel.

**Caching.** `repo.WithCache(dc).ListQueryResultsByQuery(q)` caches a Complete result under the
whole query, time range included. That is right for a closed range in the past and wrong for one
ending now: the cache keeps answering with the rows of the first run until the TTL lapses.

## Reading the results

`QueryResults` holds `Rows`, `Status`, `QueryID`, `QueryLanguage`, the requested `Limit` and the
run's `Statistics`. `Statistics.BytesScanned` is what the query is billed on.

A `QueryRow` keeps its fields in the order AWS returned them:

- `Lookup(name)` tells an absent field from an empty one — Logs Insights leaves out a field the
  event does not have.
- `Get(name)`, `Map()`, `GetMessage()`, `GetPtr()`.
- `GetTimestamp()` parses `@timestamp`, which Logs Insights renders in UTC.

`@ptr` is present on rows backed by a single log event; aggregation (`stats`) rows have none.

## How it behaves

- **Only a Complete query returns a nil error.** Failed, Cancelled, Timeout and Unknown come back
  as a `*QueryFailedError` (reach it with `errors.As`) along with the status. AWS stops a query
  after 60 minutes; narrow the range or split it if that happens.
- **A query abandoned on the way is stopped.** When the context ends or a poll fails,
  `ListQueryResults*` sends `StopQuery`, on a context of its own: otherwise the query keeps
  scanning, and billing, for up to an hour. `WaitForQuery` does not stop a query it gives up on;
  whoever started it decides.
- **Rows stream only once the query is Complete.** A running query does return rows, but they are
  a preview, not a prefix of the answer: a `sort` or `limit` can still evict them and a `stats`
  value keeps changing. The async variant streams page by page after completion.
- **Polls ask for one row.** A running Logs Insights QL query returns up to 10,000 preview rows on
  every poll, all of them thrown away, so polls set `MaxItems: 1` and the pages are read once the
  query is done. SQL and PPL results cannot be paged, so their polls ask for everything, as does
  the first poll of `WaitForQuery`, which does not know the query's language yet.
- **The poll delay grows** from `DefaultQueryPollMin` (500 ms) by half each poll up to
  `DefaultQueryPollMax` (5 s); `WithPollInterval(min, max)` changes both bounds.
- **The async buffer holds the whole result**: it is sized to the query's `Limit`, or 10,000
  (`QueryRowsBufferSize`, the most AWS returns without one), so the producer never waits on the
  reader and a reader that stops reading leaks no goroutine.
- `LimitReached()` is true when the query returned as many rows as its limit — more may have
  matched. It cannot tell when the limit is unknown (no `WithLimit`, or `WaitForQuery`).
- AWS caps concurrent queries per account; a fan-out past it fails `StartQuery` with
  `LimitExceededException`, which the SDK does not retry.
