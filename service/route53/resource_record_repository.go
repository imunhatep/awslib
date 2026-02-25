package route53

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
	cfg2 "github.com/imunhatep/awslib/service/cfg"
)

func (r *Route53Repository) ListResourceRecords(hostedZone HostedZone) ([]ResourceRecord, error) {
	query := &route53.ListResourceRecordSetsInput{
		HostedZoneId: hostedZone.Id,
	}

	return r.listResourceRecordsByInput(hostedZone, query)
}

func (r *Route53Repository) ListResourceRecordsByInput(query *route53.ListResourceRecordSetsInput) ([]ResourceRecord, error) {
	if query.HostedZoneId == nil || aws.ToString(query.HostedZoneId) == "" {
		return nil, errors.New("failed listing records, HostedZoneId cannot be empty")
	}

	// Fetch the hosted zone metadata so we can include it in the returned records if needed
	hostedZoneInput := &route53.GetHostedZoneInput{Id: query.HostedZoneId}
	hostedZone, err := r.GetHostedZoneByInput(hostedZoneInput)
	if err != nil {
		return nil, errors.New(err)
	}

	if hostedZone == nil {
		return []ResourceRecord{}, errors.New("failed listing records, HostedZone not found")
	}

	return r.listResourceRecordsByInput(*hostedZone, query)
}

func (r *Route53Repository) listResourceRecordsByInput(hostedZone HostedZone, query *route53.ListResourceRecordSetsInput) ([]ResourceRecord, error) {
	start := time.Now()

	if query.HostedZoneId == nil || aws.ToString(query.HostedZoneId) == "" {
		return nil, errors.New("failed listing records, HostedZoneId cannot be empty")
	}

	resourceRecords := []ResourceRecord{}

	// Work on a copy of the input so we can update StartRecord* for pagination
	nextQuery := *query
	for {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequests.With(r.promLabels("ListResourceRecordSets", cfg2.ResourceTypeRoute53ResourceRecord)).Inc()
		}

		output, err := r.client.Route53().ListResourceRecordSets(r.ctx, &nextQuery)
		if err != nil {
			return nil, errors.New(err)
		}

		for _, rec := range output.ResourceRecordSets {
			// Use constructor to properly populate AbstractResource and embed the AWS ResourceRecordSet
			rr := NewResourceRecord(r.client, hostedZone, rec)
			resourceRecords = append(resourceRecords, rr)
		}

		if !output.IsTruncated {
			break
		}

		// Prepare nextQuery to continue from the next record
		nextQuery.StartRecordName = output.NextRecordName
		nextQuery.StartRecordType = output.NextRecordType
		nextQuery.StartRecordIdentifier = output.NextRecordIdentifier
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("ListResourceRecordSets", cfg2.ResourceTypeRoute53ResourceRecord)).
			Add(float64(len(resourceRecords)))

		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListResourceRecordsByInput", cfg2.ResourceTypeRoute53ResourceRecord)).
			Observe(time.Since(start).Seconds())
	}

	return resourceRecords, nil
}

// ChangeResourceRecordSetsByInput performs batch changes (create, upsert, delete) on resource records
func (r *Route53Repository) ChangeResourceRecordSetsByInput(input *route53.ChangeResourceRecordSetsInput) (*route53.ChangeResourceRecordSetsOutput, error) {
	start := time.Now()

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.With(r.promLabels("ChangeResourceRecordSets", cfg2.ResourceTypeRoute53ResourceRecord)).Inc()
	}

	output, err := r.client.Route53().ChangeResourceRecordSets(r.ctx, input)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.With(r.promLabels("ChangeResourceRecordSets", cfg2.ResourceTypeRoute53ResourceRecord)).Inc()
		}

		return nil, errors.New(err)
	}

	if metrics.AwsMetricsEnabled {
		changeCount := 0
		if input.ChangeBatch != nil {
			changeCount = len(input.ChangeBatch.Changes)
		}
		metrics.AwsApiResourcesFetched.With(r.promLabels("ChangeResourceRecordSets", cfg2.ResourceTypeRoute53ResourceRecord)).Add(float64(changeCount))

		metrics.AwsRepoCallDuration.
			With(r.promLabels("ChangeResourceRecordSetsByInput", cfg2.ResourceTypeRoute53ResourceRecord)).
			Observe(time.Since(start).Seconds())
	}

	return output, nil
}

// CreateResourceRecord creates new resource records with CREATE action
// Smartly initializes input if nil or adds to existing changes
func (r *Route53Repository) CreateResourceRecord(hostedZoneId *string, recordSets ...types.ResourceRecordSet) (*route53.ChangeResourceRecordSetsOutput, error) {
	if hostedZoneId == nil || aws.ToString(hostedZoneId) == "" {
		return nil, errors.New("hostedZoneId cannot be empty")
	}

	if len(recordSets) == 0 {
		return nil, errors.New("at least one resource record set is required")
	}

	changes := make([]types.Change, 0, len(recordSets))
	for _, recordSet := range recordSets {
		rs := recordSet // capture loop variable
		changes = append(changes, types.Change{
			Action:            types.ChangeActionCreate,
			ResourceRecordSet: &rs,
		})
	}

	input := &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: hostedZoneId,
		ChangeBatch: &types.ChangeBatch{
			Changes: changes,
		},
	}

	return r.ChangeResourceRecordSetsByInput(input)
}

// UpsertResourceRecord updates (upserts) resource records with UPSERT action
// Smartly initializes input if nil or adds to existing changes
func (r *Route53Repository) UpsertResourceRecord(hostedZoneId *string, recordSets ...types.ResourceRecordSet) (*route53.ChangeResourceRecordSetsOutput, error) {
	if hostedZoneId == nil || aws.ToString(hostedZoneId) == "" {
		return nil, errors.New("hostedZoneId cannot be empty")
	}

	if len(recordSets) == 0 {
		return nil, errors.New("at least one resource record set is required")
	}

	changes := make([]types.Change, 0, len(recordSets))
	for _, recordSet := range recordSets {
		rs := recordSet // capture loop variable
		changes = append(changes, types.Change{
			Action:            types.ChangeActionUpsert,
			ResourceRecordSet: &rs,
		})
	}

	input := &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: hostedZoneId,
		ChangeBatch: &types.ChangeBatch{
			Changes: changes,
		},
	}

	return r.ChangeResourceRecordSetsByInput(input)
}

// DeleteResourceRecord deletes resource records with DELETE action
// Smartly initializes input if nil or adds to existing changes
func (r *Route53Repository) DeleteResourceRecord(hostedZoneId *string, recordSets ...types.ResourceRecordSet) error {
	if hostedZoneId == nil || aws.ToString(hostedZoneId) == "" {
		return errors.New("hostedZoneId cannot be empty")
	}

	if len(recordSets) == 0 {
		return errors.New("at least one resource record set is required")
	}

	changes := make([]types.Change, 0, len(recordSets))
	for _, recordSet := range recordSets {
		rs := recordSet // capture loop variable
		changes = append(changes, types.Change{
			Action:            types.ChangeActionDelete,
			ResourceRecordSet: &rs,
		})
	}

	input := &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: hostedZoneId,
		ChangeBatch: &types.ChangeBatch{
			Changes: changes,
		},
	}

	_, err := r.ChangeResourceRecordSetsByInput(input)
	return err
}
