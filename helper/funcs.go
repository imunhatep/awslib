package helper

import (
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	ptypes "github.com/imunhatep/awslib/provider/types"
)

//arn:aws:iam::854502996645:user/shared-terraform-iam-readonly
func BuildArn(accountId ptypes.AwsAccountID, region ptypes.AwsRegion, resource, prefix string, name *string) *arn.ARN {
	rArn, err := arn.Parse(fmt.Sprintf("arn:aws:%s:%s:%s:%s%s", resource, region, accountId, prefix, aws.ToString(name)))
	if err != nil {
		return nil
	}

	return &rArn
}
