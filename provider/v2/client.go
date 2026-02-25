package v2

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/accessanalyzer"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	"github.com/aws/aws-sdk-go-v2/service/apigateway"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/batch"
	"github.com/aws/aws-sdk-go-v2/service/cloudcontrol"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/efs"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/emr"
	"github.com/aws/aws-sdk-go-v2/service/emrserverless"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	"github.com/aws/aws-sdk-go-v2/service/health"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/pricing"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3control"
	"github.com/aws/aws-sdk-go-v2/service/s3outposts"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/storagegateway"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/aws-sdk-go-v2/service/swf"
	"github.com/aws/aws-sdk-go-v2/service/synthetics"
	"github.com/aws/aws-sdk-go-v2/service/timestreamwrite"
	"github.com/aws/aws-sdk-go-v2/service/transfer"
	"github.com/aws/aws-sdk-go-v2/service/waf"
	"github.com/aws/aws-sdk-go-v2/service/wafregional"
	"github.com/aws/aws-sdk-go-v2/service/wafv2"
	"github.com/imunhatep/awslib/provider/types"
	v3 "github.com/imunhatep/awslib/provider/v3"

	// Import v3 service clients
	v3accessanalyzer "github.com/imunhatep/awslib/provider/v3/clients/accessanalyzer"
	v3acm "github.com/imunhatep/awslib/provider/v3/clients/acm"
	v3apigateway "github.com/imunhatep/awslib/provider/v3/clients/apigateway"
	v3athena "github.com/imunhatep/awslib/provider/v3/clients/athena"
	v3autoscaling "github.com/imunhatep/awslib/provider/v3/clients/autoscaling"
	v3batch "github.com/imunhatep/awslib/provider/v3/clients/batch"
	v3cloudcontrol "github.com/imunhatep/awslib/provider/v3/clients/cloudcontrol"
	v3cloudformation "github.com/imunhatep/awslib/provider/v3/clients/cloudformation"
	v3cloudtrail "github.com/imunhatep/awslib/provider/v3/clients/cloudtrail"
	v3cloudwatch "github.com/imunhatep/awslib/provider/v3/clients/cloudwatch"
	v3cloudwatchlogs "github.com/imunhatep/awslib/provider/v3/clients/cloudwatchlogs"
	v3costexplorer "github.com/imunhatep/awslib/provider/v3/clients/costexplorer"
	v3dynamodb "github.com/imunhatep/awslib/provider/v3/clients/dynamodb"
	v3ec2 "github.com/imunhatep/awslib/provider/v3/clients/ec2"
	v3ecs "github.com/imunhatep/awslib/provider/v3/clients/ecs"
	v3efs "github.com/imunhatep/awslib/provider/v3/clients/efs"
	v3eks "github.com/imunhatep/awslib/provider/v3/clients/eks"
	v3elasticache "github.com/imunhatep/awslib/provider/v3/clients/elasticache"
	v3elasticloadbalancingv2 "github.com/imunhatep/awslib/provider/v3/clients/elasticloadbalancingv2"
	v3emr "github.com/imunhatep/awslib/provider/v3/clients/emr"
	v3emrserverless "github.com/imunhatep/awslib/provider/v3/clients/emrserverless"
	v3glue "github.com/imunhatep/awslib/provider/v3/clients/glue"
	v3health "github.com/imunhatep/awslib/provider/v3/clients/health"
	v3iam "github.com/imunhatep/awslib/provider/v3/clients/iam"
	v3lambda "github.com/imunhatep/awslib/provider/v3/clients/lambda"
	v3pricing "github.com/imunhatep/awslib/provider/v3/clients/pricing"
	v3rds "github.com/imunhatep/awslib/provider/v3/clients/rds"
	v3route53 "github.com/imunhatep/awslib/provider/v3/clients/route53"
	v3s3 "github.com/imunhatep/awslib/provider/v3/clients/s3"
	v3secretsmanager "github.com/imunhatep/awslib/provider/v3/clients/secretsmanager"
	v3storagegateway "github.com/imunhatep/awslib/provider/v3/clients/storagegateway"
	v3swf "github.com/imunhatep/awslib/provider/v3/clients/swf"
	v3synthetics "github.com/imunhatep/awslib/provider/v3/clients/synthetics"
	v3timestreamwrite "github.com/imunhatep/awslib/provider/v3/clients/timestreamwrite"
	v3transfer "github.com/imunhatep/awslib/provider/v3/clients/transfer"
	v3waf "github.com/imunhatep/awslib/provider/v3/clients/waf"
	v3wafregional "github.com/imunhatep/awslib/provider/v3/clients/wafregional"
	v3wafv2 "github.com/imunhatep/awslib/provider/v3/clients/wafv2"
)

type Client struct {
	v3Client *v3.Client
}

func NewClient(ctx context.Context, providers ...func(*config.LoadOptions) error) (*Client, error) {
	v3Client, err := v3.NewClient(ctx, providers...)
	if err != nil {
		return nil, err
	}

	return &Client{
		v3Client: v3Client,
	}, nil
}

func (c *Client) GetRegion() types.AwsRegion {
	return c.v3Client.GetRegion()
}

func (c *Client) GetAccountID() types.AwsAccountID {
	return c.v3Client.GetAccountID()
}

func (c *Client) GetCallerIdentity(ctx context.Context) (*sts.GetCallerIdentityOutput, error) {
	return c.v3Client.GetCallerIdentity(ctx)
}

// AccessAnalyzer returns a client for AWS Access Analyzer
func (c *Client) AccessAnalyzer() *accessanalyzer.Client {
	return v3accessanalyzer.GetClient(c.v3Client)
}

// ACM returns a client for AWS Certificate Manager
func (c *Client) ACM() *acm.Client {
	return v3acm.GetClient(c.v3Client)
}

// ApiGateway returns a client for AWS ApiGateway service
func (c *Client) ApiGateway() *apigateway.Client {
	return v3apigateway.GetClient(c.v3Client)
}

// Athena returns a client for AWS Athena service
func (c *Client) Athena() *athena.Client {
	return v3athena.GetClient(c.v3Client)
}

// Autoscaling returns a client for AWS Autoscaling service
func (c *Client) Autoscaling() *autoscaling.Client {
	return v3autoscaling.GetClient(c.v3Client)
}

// Batch returns a client for AWS Batch service
func (c *Client) Batch() *batch.Client {
	return v3batch.GetClient(c.v3Client)
}

// CloudControl returns a client for AWS CloudControl service
func (c *Client) CloudControl() *cloudcontrol.Client {
	return v3cloudcontrol.GetClient(c.v3Client)
}

// Cloudformation returns a client for AWS Cloudformation service
func (c *Client) Cloudformation() *cloudformation.Client {
	return v3cloudformation.GetClient(c.v3Client)
}

// CloudTrail returns a client for AWS CloudTrail service
func (c *Client) CloudTrail() *cloudtrail.Client {
	return v3cloudtrail.GetClient(c.v3Client)
}

// CloudWatch returns a client for AWS CloudWatch service
func (c *Client) CloudWatch() *cloudwatch.Client {
	return v3cloudwatch.GetClient(c.v3Client)
}

// CloudWatchLogs returns a client for AWS CloudWatchLogs service
func (c *Client) CloudWatchLogs() *cloudwatchlogs.Client {
	return v3cloudwatchlogs.GetClient(c.v3Client)
}

func (c *Client) Costexplorer() *costexplorer.Client {
	return v3costexplorer.GetClient(c.v3Client)
}

// DynamoDB returns a client for AWS DynamoDB service
func (c *Client) DynamoDB() *dynamodb.Client {
	return v3dynamodb.GetClient(c.v3Client)
}

// EC2 returns a client for AWS EC2 service
func (c *Client) EC2() *ec2.Client {
	return v3ec2.GetClient(c.v3Client)
}

// ECS returns a client for AWS ECS service
func (c *Client) ECS() *ecs.Client {
	return v3ecs.GetClient(c.v3Client)
}

// EFS returns a client for AWS EFS service
func (c *Client) EFS() *efs.Client {
	return v3efs.GetClient(c.v3Client)
}

// Elasticloadbalancingv2 returns a client for AWS Elasticloadbalancingv2 service
func (c *Client) ELBv2() *elasticloadbalancingv2.Client {
	return v3elasticloadbalancingv2.GetClient(c.v3Client)
}

// EKS returns a client for AWS EKS service
func (c *Client) EKS() *eks.Client {
	return v3eks.GetClient(c.v3Client)
}

// Elasticache returns a client for AWS Elasticache service
func (c *Client) Elasticache() *elasticache.Client {
	return v3elasticache.GetClient(c.v3Client)
}

// EMR returns a client for AWS EMR service
func (c *Client) EMR() *emr.Client {
	return v3emr.GetClient(c.v3Client)
}

// EMRServerless returns a client for AWS EMRServerless service
func (c *Client) EMRServerless() *emrserverless.Client {
	return v3emrserverless.GetClient(c.v3Client)
}

// Glue returns a client for AWS Glue service
func (c *Client) Glue() *glue.Client {
	return v3glue.GetClient(c.v3Client)
}

// Health returns a client for AWS Health service
func (c *Client) Health() *health.Client {
	return v3health.GetClient(c.v3Client)
}

// IAM returns a client for AWS IAM service
func (c *Client) IAM() *iam.Client {
	return v3iam.GetClient(c.v3Client)
}

// Lambda returns a client for AWS Lambda service
func (c *Client) Lambda() *lambda.Client {
	return v3lambda.GetClient(c.v3Client)
}

// Pricing returns a client for AWS Pricing service
func (c *Client) Pricing() *pricing.Client {
	return v3pricing.GetClient(c.v3Client)
}

// Route53 returns a client for AWS Route53 service
func (c *Client) Route53() *route53.Client {
	return v3route53.GetClient(c.v3Client)
}

// RDS returns a client for AWS RDS service
func (c *Client) RDS() *rds.Client {
	return v3rds.GetClient(c.v3Client)
}

// SecretsManager returns a client for AWS S3 service
func (c *Client) SecretsManager() *secretsmanager.Client {
	return v3secretsmanager.GetClient(c.v3Client)
}

// S3 returns a client for AWS S3 service
func (c *Client) S3(optFns ...func(o *s3.Options)) *s3.Client {
	return v3s3.GetClient(c.v3Client, optFns...)
}

// S3Control returns a client for AWS S3Control service
func (c *Client) S3Control() *s3control.Client {
	return s3control.NewFromConfig(c.v3Client.Config())
}

// S3Outposts returns a client for AWS S3Outposts service
func (c *Client) S3Outposts() *s3outposts.Client {
	return s3outposts.NewFromConfig(c.v3Client.Config())
}

// SNS returns a client for AWS SNS service
func (c *Client) SNS() *sns.Client {
	return sns.NewFromConfig(c.v3Client.Config())
}

// SQS returns a client for AWS SQS service
func (c *Client) SQS() *sqs.Client {
	return sqs.NewFromConfig(c.v3Client.Config())
}

// SSM returns a client for AWS SSM service
func (c *Client) SSM() *ssm.Client {
	return ssm.NewFromConfig(c.v3Client.Config())
}

// StorageGateway returns a client for AWS StorageGateway service
func (c *Client) StorageGateway() *storagegateway.Client {
	return v3storagegateway.GetClient(c.v3Client)
}

// SWF returns a client for AWS SWF service
func (c *Client) SWF() *swf.Client {
	return v3swf.GetClient(c.v3Client)
}

// Sts returns a client for AWS Security Token Service
func (c *Client) Sts() *sts.Client {
	return sts.NewFromConfig(c.v3Client.Config())
}

// Synthetics returns a client for AWS Synthetics service
func (c *Client) Synthetics() *synthetics.Client {
	return v3synthetics.GetClient(c.v3Client)
}

// TimestreamWrite returns a client for AWS TimestreamWrite service
func (c *Client) TimestreamWrite() *timestreamwrite.Client {
	return v3timestreamwrite.GetClient(c.v3Client)
}

// Transfer returns a client for AWS Transfer service
func (c *Client) Transfer() *transfer.Client {
	return v3transfer.GetClient(c.v3Client)
}

// WAF returns a client for AWS WAF service
func (c *Client) WAF() *waf.Client {
	return v3waf.GetClient(c.v3Client)
}

// WAFRegional returns a client for AWS WAFRegional service
func (c *Client) WAFRegional() *wafregional.Client {
	return v3wafregional.GetClient(c.v3Client)
}

// WAFv2 returns a client for AWS WAFv2 service
func (c *Client) WAFv2() *wafv2.Client {
	return v3wafv2.GetClient(c.v3Client)
}
