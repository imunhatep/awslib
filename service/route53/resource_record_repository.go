package route53

import (
	"strings"
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

		output, err := r.route53Client().ListResourceRecordSets(r.ctx, &nextQuery)
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

	output, err := r.route53Client().ChangeResourceRecordSets(r.ctx, input)
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
// Automatically normalizes record names and ensures proper FQDN format
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
		// Automatically normalize and ensure FQDN for record name
		if rs.Name != nil {
			normalizedName := NormalizeRoute53Name(aws.ToString(rs.Name))
			fqdnName := EnsureFQDN(normalizedName)
			rs.Name = aws.String(fqdnName)
		}
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
// Automatically normalizes record names and ensures proper FQDN format
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
		// Automatically normalize and ensure FQDN for record name
		if rs.Name != nil {
			normalizedName := NormalizeRoute53Name(aws.ToString(rs.Name))
			fqdnName := EnsureFQDN(normalizedName)
			rs.Name = aws.String(fqdnName)
		}
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
// Automatically normalizes record names and ensures proper FQDN format
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
		// Automatically normalize and ensure FQDN for record name
		if rs.Name != nil {
			normalizedName := NormalizeRoute53Name(aws.ToString(rs.Name))
			fqdnName := EnsureFQDN(normalizedName)
			rs.Name = aws.String(fqdnName)
		}
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

// NormalizeRoute53Name handles escaped characters in Route53 record names
// AWS Route53 API sometimes returns special characters like asterisk as octal escapes (\052)
// This function is exported so it can be used by applications when needed
// Note: This normalization is automatically applied in repository CRUD operations
func NormalizeRoute53Name(name string) string {
	normalized := strings.TrimSpace(name)
	// Handle asterisk character which is commonly escaped as \052 (octal)
	normalized = strings.ReplaceAll(normalized, "\\052", "*")
	// Handle other common escaped characters if they appear
	normalized = strings.ReplaceAll(normalized, "\\\\052", "*") // double-escaped
	normalized = strings.ReplaceAll(normalized, "\\\\", "\\")   // double backslashes
	return normalized
}

// EnsureFQDN ensures the domain name ends with a dot to be a proper FQDN
// This is a utility function that can be used when creating Route53 records
// Note: This FQDN formatting is automatically applied in repository CRUD operations
func EnsureFQDN(name string) string {
	if !strings.HasSuffix(name, ".") {
		return name + "."
	}
	return name
}
