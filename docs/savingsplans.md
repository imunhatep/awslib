# Savings Plans

`service/savingsplans` reads two different things:

- **Inventory** — the Savings Plans an account has already purchased
  (`DescribeSavingsPlans`).
- **Offering rates** — what a commitment *would* cost, per region, product and instance
  type (`DescribeSavingsPlansOfferingRates`).

Neither is a resource. A purchased plan is closer to a contract and a rate is a price,
so nothing here implements `service.ResourceInterface`, nothing is reachable through
`proxy.RepoProxy.FindAll`, and the two `cfg.ResourceType` constants
(`ResourceTypeSavingsPlan`, `ResourceTypeSavingsPlanOfferingRate`) exist only to label
metrics — they are deliberately absent from `cfg.ResourceTypeList`.

## Region is a filter, not an endpoint

The Savings Plans API is partition-global: every client resolves to
`savingsplans.amazonaws.com` whatever region it was built for. So the region of the
client says nothing about the answer — inventory is account-wide, and offering rates
take the region as a **query filter**.

Never infer a region from the repository you hold. Put it in `OfferingRatesQuery.Region`
and one client can price every region.

## Offering rates

```go
package main

import (
    "context"
    "fmt"

    sptypes "github.com/aws/aws-sdk-go-v2/service/savingsplans/types"
    ptypes "github.com/imunhatep/awslib/provider/types"
    v3 "github.com/imunhatep/awslib/provider/v3"
    "github.com/imunhatep/awslib/service/savingsplans"
)

func ExampleOfferingRates(ctx context.Context, client *v3.Client) error {
    repo := savingsplans.NewSavingsPlansRepository(ctx, client)

    // What a 1-year, no-upfront Compute Savings Plan costs for Linux EC2 in Frankfurt.
    rates, err := repo.ListOfferingRatesByQuery(savingsplans.OfferingRatesQuery{
        Region:              ptypes.AwsRegion("eu-central-1"),
        PlanTypes:           []sptypes.SavingsPlanType{sptypes.SavingsPlanTypeCompute},
        PaymentOptions:      []sptypes.SavingsPlanPaymentOption{sptypes.SavingsPlanPaymentOptionNoUpfront},
        Products:            []sptypes.SavingsPlanProductType{sptypes.SavingsPlanProductTypeEc2},
        ServiceCodes:        []sptypes.SavingsPlanRateServiceCode{sptypes.SavingsPlanRateServiceCodeEc2},
        ProductDescriptions: []string{"Linux/UNIX"},
        Tenancies:           []string{"shared"},
        DurationSeconds:     savingsplans.Term1yrSeconds,
    })
    if err != nil {
        return err
    }

    for _, rate := range rates {
        fmt.Printf("%s in %s: %s $/h\n", rate.InstanceType(), rate.Region(), rate.HourlyRate())
    }

    return nil
}
```

One call returns every instance type in that region for that commitment, so pricing a
fleet costs one request per region and plan type — not one per instance type.

### The query

Everything except `DurationSeconds` is applied by AWS. `DurationSeconds` is applied
**client-side**, because the commitment length is carried by each result's parent
offering and the API offers no filter for it. Zero keeps every term.

| Field | Applied by | Notes |
|---|---|---|
| `Region` | AWS | empty means every region |
| `PlanTypes` | AWS | `Compute`, `EC2Instance`, … |
| `PaymentOptions` | AWS | `No Upfront`, `Partial Upfront`, `All Upfront` |
| `Products`, `ServiceCodes` | AWS | scope to one AWS service |
| `Operations`, `UsageTypes` | AWS | billing-report identifiers |
| `InstanceTypes`, `InstanceFamilies` | AWS | |
| `ProductDescriptions`, `Tenancies` | AWS | e.g. `Linux/UNIX`, `shared` |
| `DurationSeconds` | client-side | `Term1yrSeconds`, `Term3yrSeconds` |
| `PageSize` | AWS | defaults to `DefaultOfferingRatePageSize` (1000) |

Filters the caller leaves empty are omitted from the request entirely — AWS rejects an
empty filter element rather than ignoring it.

### Reading a rate

`OfferingRate` embeds the SDK type, so nothing is lost, and adds accessors for the parts
that are awkward to reach: the commitment lives on a parent offering behind a pointer,
and what a rate applies to arrives as untyped name/value properties.

```go
rate.HourlyRate()         // "0.0680", "" when absent
rate.DurationSeconds()    // Term1yrSeconds
rate.PlanType()           // sptypes.SavingsPlanTypeCompute
rate.PaymentOption()      // "No Upfront"
rate.Currency()           // "USD"
rate.InstanceType()       // "m5.large"
rate.InstanceFamily()     // "m5"
rate.Region()             // "eu-central-1"
rate.ProductDescription() // "Linux/UNIX"
rate.Tenancy()            // "shared"
rate.Property(savingsplans.PropertyInstanceType) // any property by name
```

Every accessor is nil-safe: a rate AWS returned without a parent offering reads as zero
values rather than panicking. A property the rate does not carry returns `""` — ordinary,
since a rate only carries the properties that apply to its product type.

## Purchased plans

```go
plans, err := repo.ListSavingsPlansByStates(
    []sptypes.SavingsPlanState{savingsplans.SavingsPlanStateActive},
)
if err != nil {
    return err
}

for _, plan := range plans {
    fmt.Println(aws.ToString(plan.SavingsPlanId), plan.SavingsPlanType, aws.ToString(plan.Commitment))
}
```

`ListSavingsPlansAll()` returns every plan in any state. Plans are account-wide, so
calling either once per region multiplies requests without finding a single extra plan —
one client per account is enough.

## Caching

`WithCache` is generated like every other repository, namespaced
`<accountID>:<region>:savingsplans`:

```go
repo := savingsplans.NewSavingsPlansRepository(ctx, client).WithCache(dataCache)
```

Both the query struct and the raw SDK input are rendered by value into the cache key, so
two different commitments never share an entry.

Worth deciding per call site: offering rates are published prices that change rarely and
cache well, while inventory answers "what do we own right now" — a stale answer there
misleads rather than merely lags.

## Pagination

Neither API ships an SDK paginator, so both `List*` methods drive `NextToken` internally
and return complete, flattened slices. The caller never sees a token.
