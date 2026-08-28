# Cost Explorer

`service/costexplorer` wraps the Cost Explorer API: cost and usage figures, forecasts and
dimension values. It is not a resource repository — costs are not resources, so nothing here
flows through `proxy.RepoProxy` or the parallel fetcher. You call it directly.

What it adds over `costexplorer.GetClient(client)`:

- **`NextPageToken` pagination** followed to the end and accumulated into one `CostAndUsage`.
- **`CostQuery`**, a console-shaped request — a window, a granularity, metrics, filter rows
  and groupings — validated before the round trip.
- **A filter and grouping DSL** (`ByDimension`/`ByTag`/`ByCostCategory`, `And`/`Or`/`Not`,
  `FilterByService`, `GroupByTag`, …) instead of hand-nested `types.Expression` trees.
- **Caching** via `WithCache(dc)` and Prometheus metrics on every call.

Two things to know before the first call:

- **It costs money.** Every request is billed ($0.01 per paginated request at the time of
  writing), which is what makes `WithCache` more than a latency optimisation.
- **The endpoint is global.** Cost Explorer answers from `us-east-1` regardless of the
  client's region. Build **one** repository, in the payer account, and split by account with
  `GroupByLinkedAccount()` — do not fan out across regions or accounts, that just buys the
  same numbers several times.

## A first query

```go
import (
    "time"

    cetypes "github.com/aws/aws-sdk-go-v2/service/costexplorer/types"
    "github.com/imunhatep/awslib/service/costexplorer"
)

repo := costexplorer.NewCostExplorerRepository(ctx, client)

end := time.Now().Truncate(24 * time.Hour)
start := end.AddDate(0, 0, -30)

result, err := repo.GetCostAndUsageByQuery(costexplorer.CostQuery{
    Start:       start,
    End:         end, // exclusive
    Granularity: cetypes.GranularityDaily,
    Metrics:     []string{costexplorer.MetricUnblendedCost},
    GroupBy:     []cetypes.GroupDefinition{costexplorer.GroupByService()},
})
if err != nil {
    return err
}

for _, period := range result.GetResultsByTime() {
    for _, group := range period.Groups {
        fmt.Println(*period.TimePeriod.Start, group.Keys, *group.Metrics["UnblendedCost"].Amount)
    }
}
```

`Start` is inclusive and `End` is exclusive, as the API requires. The window is rendered in
the layout the granularity expects — `2006-01-02` for `DAILY`/`MONTHLY`, a full
`2006-01-02T15:04:05Z` timestamp for `HOURLY` — so pass `time.Time` values and let the
query do it.

Without a `GroupBy`, the per-period `Total` map is populated instead of `Groups`, and the
whole window collapses to one number with:

```go
amount, unit := result.GetTotalByMetric(costexplorer.MetricUnblendedCost)
```

`GetCostAndUsageByPeriod(start, end, granularity, metrics, groupBy)` is the same thing with
no filtering, for when a struct literal is more ceremony than the call deserves.

## Metrics

| Constant | Value |
|---|---|
| `costexplorer.MetricUnblendedCost` | `UnblendedCost` — what the console shows by default |
| `costexplorer.MetricAmortizedCost` | `AmortizedCost` — spreads upfront RI/SP charges |
| `costexplorer.MetricBlendedCost` | `BlendedCost` |
| `costexplorer.MetricNetUnblendedCost` | `NetUnblendedCost` — after discounts |
| `costexplorer.MetricNetAmortizedCost` | `NetAmortizedCost` |
| `costexplorer.MetricUsageQuantity` | `UsageQuantity` |
| `costexplorer.MetricNormalizedUsage` | `NormalizedUsageAmount` |

Several can be requested at once; each shows up as its own key in the returned metric maps.
`UsageQuantity` mixes units across usage types, so it only means something alongside a
filter or a `GroupByUsageType()`.

## Filtering

`CostQuery` has four ways in, and they are all AND-ed together:

```go
q := costexplorer.CostQuery{
    Start:       start,
    End:         end,
    Granularity: cetypes.GranularityMonthly,
    Metrics:     []string{costexplorer.MetricAmortizedCost},

    // 1. shorthand: SERVICE dimension
    Services: []string{"Amazon Elastic Compute Cloud - Compute", "Amazon Simple Storage Service"},

    // 2. shorthand: cost allocation tags, key -> allowed values
    Tags: map[string][]string{"Team": {"platform", "data"}},

    // 3. filter rows, one per criterion
    Filters: []costexplorer.Filter{
        costexplorer.ByDimension(cetypes.DimensionRecordType, "Usage"),
        costexplorer.ByDimension(cetypes.DimensionRegion, "eu-central-1", "eu-west-1"),
        costexplorer.ByTag("Environment", "production").Excluded(),
    },

    // 4. an escape hatch for a hand-built expression
    Filter: costexplorer.Or(
        costexplorer.FilterByInstanceTypeFamily("m6i"),
        costexplorer.FilterByInstanceTypeFamily("m7i"),
    ),
}
```

The semantics match the console's filter panel: **values inside one row are OR-ed**, **rows
are AND-ed**. A row builder is a value, so it chains:

| Builder | Modifiers |
|---|---|
| `ByDimension(dim, values...)` | `.Matching(opts...)`, `.Excluded()` |
| `ByTag(key, values...)` | `.Matching(opts...)`, `.Excluded()`, `.Present()`, `.Absent()` |
| `ByCostCategory(key, values...)` | `.Matching(opts...)`, `.Excluded()`, `.Present()`, `.Absent()` |

`.Present()` and `.Absent()` are the "has any value" / "is untagged" rows the console offers
— which is how you find spend that no team owns:

```go
Filters: []costexplorer.Filter{costexplorer.ByTag("Team").Absent()}
```

`.Matching()` sets the operator, and `GetCostAndUsage` is stricter than the API's type
system: **dimensions accept only `EQUALS` and `CASE_SENSITIVE`**; tags and cost categories
also accept `ABSENT`. `CONTAINS`, `STARTS_WITH` and friends belong to cost category rules,
anomaly subscriptions and `GetDimensionValues`. `Validate()` catches the mistake locally
rather than after a billed round trip.

`Filter` has an unexported validation method, so a row type cannot be defined outside the
package — the three builders are the whole set. Anything else goes through the
`Filter *types.Expression` field.

### Ready-made expressions

For the `Filter` escape hatch, or anywhere you are assembling expressions by hand, the
package ships one constructor per dimension so you never spell a dimension name:

`FilterByService`, `FilterByLinkedAccount`, `FilterByRegion`, `FilterByAvailabilityZone`,
`FilterByInstanceType`, `FilterByInstanceTypeFamily`, `FilterByUsageType`,
`FilterByUsageTypeGroup`, `FilterByChargeType`, `FilterByPurchaseType`, `FilterByOperation`,
`FilterByResourceID`, `FilterByPlatform`, `FilterByOperatingSystem`, `FilterByTenancy`,
`FilterByDatabaseEngine`, `FilterByDeploymentOption`, `FilterByCacheEngine`,
`FilterByBillingEntity`, `FilterByLegalEntity`, `FilterByInvoicingEntity`, `FilterByScope`,
`FilterByPaymentOption`, `FilterBySavingsPlansType`, `FilterBySavingsPlanArn`,
`FilterByReservationID`, `FilterBySubscriptionID`, plus
`FilterByDimension(dimension, values...)` for anything not listed.

Tags and cost categories: `FilterByTag(key, values...)`, `FilterByTagPresent(key)`,
`FilterByTagAbsent(key)`, `FilterByTags(map[string][]string)`, and the
`FilterByCostCategory*` equivalents.

Combinators: `And(exprs...)`, `Or(exprs...)`, `Not(expr)`, and `FiltersExpression(rows...)`
to fold a slice of `Filter` rows into one expression. All of them ignore `nil` arguments and
return `nil` when nothing is left, so an unset filter simply disappears instead of producing
an empty expression the API rejects — which is why `FilterByService(q.Services...)` is safe
with an empty slice.

## Grouping

`GetCostAndUsage` accepts at most **two** groupings per request
(`costexplorer.MaxGroupBy`), counting all types together. `Validate()` enforces it.

```go
GroupBy: []cetypes.GroupDefinition{
    costexplorer.GroupByLinkedAccount(),
    costexplorer.GroupByService(),
}
```

Available: `GroupByService`, `GroupByLinkedAccount`, `GroupByRegion`, `GroupByInstanceType`,
`GroupByUsageType`, `GroupByChargeType`, `GroupByPurchaseType`, `GroupByOperation`,
`GroupByDimension(dim)`, `GroupByTag(key)`, `GroupByCostCategory(key)`, and the plural
`GroupByTags(keys...)` / `GroupByCostCategories(keys...)` which return a slice — still
subject to the limit of two.

Group keys come back in `period.Groups[i].Keys`, in the order the groupings were given.
Tag groupings key as `Team$platform` (and `Team$` for resources without the tag), which is
worth knowing before splitting on `$`.

## Forecast and dimension values

`GetCostForecast` and `GetDimensionValues` take the raw SDK inputs — there is no
higher-level wrapper, because both are single-shot questions:

```go
import awsce "github.com/aws/aws-sdk-go-v2/service/costexplorer"

forecast, err := repo.GetCostForecast(&awsce.GetCostForecastInput{
    TimePeriod:  &cetypes.DateInterval{Start: aws.String("2026-09-01"), End: aws.String("2026-10-01")},
    Granularity: cetypes.GranularityMonthly,
    Metric:      cetypes.MetricUnblendedCost,
})

// What values can I actually filter on?
values, err := repo.GetDimensionValues(&awsce.GetDimensionValuesInput{
    Dimension:  cetypes.DimensionService,
    TimePeriod: &cetypes.DateInterval{Start: aws.String("2026-08-01"), End: aws.String("2026-09-01")},
})
```

Watch the name collision: `costexplorer.MetricUnblendedCost` is the string
`"UnblendedCost"` that `GetCostAndUsage` wants in `Metrics`, while
`cetypes.MetricUnblendedCost` is the SDK enum `"UNBLENDED_COST"` that `GetCostForecast`
wants in `Metric`. They are not interchangeable.

`GetDimensionValues` is the antidote to guessing service names: the `SERVICE` dimension
wants the billing spelling (`Amazon Elastic Compute Cloud - Compute`), not `ec2`. It is
paginated and accumulated like `GetCostAndUsage`. `GetCostForecast` is not paginated and
returns the SDK output unchanged; it fails outright when the history is too short or too
noisy to forecast, which is an API decision and not something this wrapper softens.

## Caching

```go
repo := costexplorer.NewCostExplorerRepository(ctx, client).WithCache(dataCache)
```

`WithCache` returns a `CostExplorerRepositoryCached` with the same five methods, namespaced
`<accountID>:<region>:costexplorer`, written only on success. Cost figures for a closed
period never change, so a long TTL is safe and directly saves billed requests; only the
current, still-accruing period benefits from a short one.

The cache key is built from the request by value (`cache.Key`), so two `CostQuery` values
that differ only in pointer identity hit the same entry. It also means a query whose window
is `time.Now()`-relative produces a fresh key on every call — round the window to the day
(as in the first example) if you want cache hits.

## IAM

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "ce:GetCostAndUsage",
        "ce:GetCostForecast",
        "ce:GetDimensionValues"
      ],
      "Resource": "*"
    }
  ]
}
```

Cost Explorer must also be **enabled** in the payer account, and cost allocation tags
activated in Billing before any `ByTag` row or `GroupByTag` returns anything — an
unactivated tag key is not an error, it simply groups everything under `Key$`.

For Savings Plans inventory and rates, see [savingsplans.md](savingsplans.md); for the
public price list, `service/pricing`.
