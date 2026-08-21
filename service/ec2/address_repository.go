package ec2

import (
	"time"

	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/metrics"
)

// ListAddressesAll returns every Elastic IP allocated in the account/region.
func (r *Ec2Repository) ListAddressesAll() ([]Address, error) {
	return r.ListAddressesByInput(&ec2.DescribeAddressesInput{})
}

// ListAddressesByInput describes Elastic IPs. DescribeAddresses is not paginated — it returns
// the full set in a single response — so there is no paginator loop here.
func (r *Ec2Repository) ListAddressesByInput(describeInput *ec2.DescribeAddressesInput) ([]Address, error) {
	if describeInput == nil {
		return []Address{}, nil
	}

	start := time.Now()
	var items []Address

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiRequests.
			With(r.promLabels("DescribeAddresses", cfg.ResourceTypeEip)).
			Inc()
	}

	resp, err := r.ec2Client().DescribeAddresses(r.ctx, describeInput)
	if err != nil {
		if metrics.AwsMetricsEnabled {
			metrics.AwsApiRequestErrors.
				With(r.promLabels("DescribeAddresses", cfg.ResourceTypeEip)).
				Inc()
		}

		return items, errors.New(err)
	}

	for _, v := range resp.Addresses {
		items = append(items, NewAddress(r.client, v))
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsApiResourcesFetched.
			With(r.promLabels("DescribeAddresses", cfg.ResourceTypeEip)).
			Add(float64(len(items)))
	}

	if metrics.AwsMetricsEnabled {
		metrics.AwsRepoCallDuration.
			With(r.promLabels("ListAddressesByInput", cfg.ResourceTypeEip)).
			Observe(time.Since(start).Seconds())
	}

	return items, nil
}
