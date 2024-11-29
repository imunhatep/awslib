package v2

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
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
	"github.com/aws/aws-sdk-go-v2/service/savingsplans"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/securityhub"
	"github.com/aws/aws-sdk-go-v2/service/servicecatalog"
	"github.com/aws/aws-sdk-go-v2/service/servicediscovery"
	"github.com/aws/aws-sdk-go-v2/service/servicequotas"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	"github.com/aws/aws-sdk-go-v2/service/shield"
	"github.com/aws/aws-sdk-go-v2/service/signer"
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
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/provider/types"
	"sync"
)

type Client struct {
	callerIdentity *sts.GetCallerIdentityOutput
	accountID      types.AwsAccountID
	region         types.AwsRegion

	cfg aws.Config
	mu  sync.Mutex // Protects client initializations

	// cached sts client for metadata
	stsClient *sts.Client
}

func NewClient(ctx context.Context, providers ...func(*config.LoadOptions) error) (*Client, error) {
	clientConf, err := config.LoadDefaultConfig(ctx, providers...)
	if err != nil {
		return nil, errors.New(err)
	}

	client := &Client{cfg: clientConf}
	client.region = types.AwsRegion(clientConf.Region)

	err = client.updateAccountID(ctx)
	if err != nil {
		return nil, errors.New(err)
	}

	return client, nil
}

func (c *Client) GetRegion() types.AwsRegion {
	return c.region
}

func (c *Client) GetAccountID() types.AwsAccountID {
	return c.accountID
}

func (c *Client) updateAccountID(ctx context.Context) error {
	if c.accountID != "" {
		return nil
	}

	callerIdentity, err := c.GetCallerIdentity(ctx)
	if err != nil {
		return errors.New(err)
	}

	c.accountID = types.AwsAccountID(aws.ToString(callerIdentity.Account))

	return nil
}

func (c *Client) GetCallerIdentity(ctx context.Context) (*sts.GetCallerIdentityOutput, error) {
	if c.callerIdentity != nil {
		return c.callerIdentity, nil
	}

	var err error
	c.callerIdentity, err = c.Sts().GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return nil, errors.New(err)
	}

	return c.callerIdentity, nil
}

// AccessAnalyzer returns a client for AWS Access Analyzer
func (c *Client) AccessAnalyzer() *accessanalyzer.Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	return accessanalyzer.NewFromConfig(c.cfg)
}

// ACM returns a client for AWS Certificate Manager
func (c *Client) ACM() *acm.Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	return acm.NewFromConfig(c.cfg)
}

// ApiGateway returns a client for AWS ApiGateway service
func (c *Client) ApiGateway() *apigateway.Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	return apigateway.NewFromConfig(c.cfg)
}

// Athena returns a client for AWS Athena service
func (c *Client) Athena() *athena.Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	return athena.NewFromConfig(c.cfg)
}

// Autoscaling returns a client for AWS Autoscaling service
func (c *Client) Autoscaling() *autoscaling.Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	return autoscaling.NewFromConfig(c.cfg)
}

// Batch returns a client for AWS Batch service
func (c *Client) Batch() *batch.Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	return batch.NewFromConfig(c.cfg)
}

// CloudControl returns a client for AWS CloudControl service
func (c *Client) CloudControl() *cloudcontrol.Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	return cloudcontrol.NewFromConfig(c.cfg)
}

// Cloudformation returns a client for AWS Cloudformation service
func (c *Client) Cloudformation() *cloudformation.Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	return cloudformation.NewFromConfig(c.cfg)
}

// CloudTrail returns a client for AWS CloudTrail service
func (c *Client) CloudTrail() *cloudtrail.Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	return cloudtrail.NewFromConfig(c.cfg)
}

// CloudWatch returns a client for AWS CloudWatch service
func (c *Client) CloudWatch() *cloudwatch.Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	return cloudwatch.NewFromConfig(c.cfg)
}

// CloudWatchLogs returns a client for AWS CloudWatchLogs service
func (c *Client) CloudWatchLogs() *cloudwatchlogs.Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	return cloudwatchlogs.NewFromConfig(c.cfg)
}

func (c *Client) Costexplorer() *costexplorer.Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	return costexplorer.NewFromConfig(c.cfg)
}

// DynamoDB returns a client for AWS DynamoDB service
func (c *Client) DynamoDB() *dynamodb.Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	return dynamodb.NewFromConfig(c.cfg)
}

// EC2 returns a client for AWS EC2 service
func (c *Client) EC2() *ec2.Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	return ec2.NewFromConfig(c.cfg)
}

// ECS returns a client for AWS ECS service
func (c *Client) ECS() *ecs.Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	return ecs.NewFromConfig(c.cfg)
}

// EFS returns a client for AWS EFS service
func (c *Client) EFS() *efs.Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	return efs.NewFromConfig(c.cfg)
}

// Elasticloadbalancingv2 returns a client for AWS Elasticloadbalancingv2 service
func (c *Client) ELBv2() *elasticloadbalancingv2.Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	return elasticloadbalancingv2.NewFromConfig(c.cfg)
}

// EKS returns a client for AWS EKS service
func (c *Client) EKS() *eks.Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	return eks.NewFromConfig(c.cfg)
}

// Elasticache returns a client for AWS Elasticache service
func (c *Client) Elasticache() *elasticache.Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	return elasticache.NewFromConfig(c.cfg)
}

// EMR returns a client for AWS EMR service
func (c *Client) EMR() *emr.Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	return emr.NewFromConfig(c.cfg)
}

// EMRServerless returns a client for AWS EMRServerless service
func (c *Client) EMRServerless() *emrserverless.Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	return emrserverless.NewFromConfig(c.cfg)
}

// Glue returns a client for AWS Glue service
func (c *Client) Glue() *glue.Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	return glue.NewFromConfig(c.cfg)
}

// Health returns a client for AWS Health service
func (c *Client) Health() *health.Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	return health.NewFromConfig(c.cfg)
}

// IAM returns a client for AWS IAM service
func (c *Client) IAM() *iam.Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	return iam.NewFromConfig(c.cfg)
}

// Lambda returns a client for AWS Lambda service
func (c *Client) Lambda() *lambda.Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	return lambda.NewFromConfig(c.cfg)
}

// Pricing returns a client for AWS Pricing service
func (c *Client) Pricing() *pricing.Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	return pricing.NewFromConfig(c.cfg)
}

// RDS returns a client for AWS RDS service
func (c *Client) Route53() *route53.Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	return route53.NewFromConfig(c.cfg)
}

// RDS returns a client for AWS RDS service
func (c *Client) RDS() *rds.Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	return rds.NewFromConfig(c.cfg)
}

// S3 returns a client for AWS S3 service
func (c *Client) S3(optFns ...func(o *s3.Options)) *s3.Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	return s3.NewFromConfig(c.cfg, optFns...)
}

// S3Control returns a client for AWS S3Control service
func (c *Client) S3Control() *s3control.Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	return s3control.NewFromConfig(c.cfg)
}

// S3Outposts returns a client for AWS S3Outposts service
func (c *Client) S3Outposts() *s3outposts.Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	return s3outposts.NewFromConfig(c.cfg)
}

// Savingsplans returns a client for AWS SNS service
func (c *Client) Savingsplans() *savingsplans.Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	return savingsplans.NewFromConfig(c.cfg)
}

// SNS returns a client for AWS SNS service
func (c *Client) SNS() *sns.Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	return sns.NewFromConfig(c.cfg)
}

// SQS returns a client for AWS SQS service
func (c *Client) SQS() *sqs.Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	return sqs.NewFromConfig(c.cfg)
}

// SSM returns a client for AWS SSM service
func (c *Client) SSM() *ssm.Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	return ssm.NewFromConfig(c.cfg)
}

// SecretsManager returns a client for AWS SecretsManager service
func (c *Client) SecretsManager() *secretsmanager.Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	return secretsmanager.NewFromConfig(c.cfg)
}

// SecurityHub returns a client for AWS SecurityHub service
func (c *Client) SecurityHub() *securityhub.Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	return securityhub.NewFromConfig(c.cfg)
}

// ServiceCatalog returns a client for AWS ServiceCatalog service
func (c *Client) ServiceCatalog() *servicecatalog.Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	return servicecatalog.NewFromConfig(c.cfg)
}

// ServiceDiscovery returns a client for AWS ServiceDiscovery service
func (c *Client) ServiceDiscovery() *servicediscovery.Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	return servicediscovery.NewFromConfig(c.cfg)
}

// ServiceQuotas returns a client for AWS ServiceQuotas service
func (c *Client) ServiceQuotas() *servicequotas.Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	return servicequotas.NewFromConfig(c.cfg)
}

// SES returns a client for AWS SES service
func (c *Client) SES() *ses.Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	return ses.NewFromConfig(c.cfg)
}

// Sfn returns a client for AWS Sfn service
func (c *Client) Sfn() *sfn.Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	return sfn.NewFromConfig(c.cfg)
}

// Shield returns a client for AWS Shield service
func (c *Client) Shield() *shield.Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	return shield.NewFromConfig(c.cfg)
}

// Signer returns a client for AWS Signer service
func (c *Client) Signer() *signer.Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	return signer.NewFromConfig(c.cfg)
}

// StorageGateway returns a client for AWS StorageGateway service
func (c *Client) StorageGateway() *storagegateway.Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	return storagegateway.NewFromConfig(c.cfg)
}

// SWF returns a client for AWS SWF service
func (c *Client) SWF() *swf.Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	return swf.NewFromConfig(c.cfg)
}

// Sts returns a client for AWS Security Token Service
func (c *Client) Sts() *sts.Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stsClient == nil {
		c.stsClient = sts.NewFromConfig(c.cfg)
	}

	return c.stsClient
}

// Synthetics returns a client for AWS Synthetics service
func (c *Client) Synthetics() *synthetics.Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	return synthetics.NewFromConfig(c.cfg)
}

// TimestreamWrite returns a client for AWS TimestreamWrite service
func (c *Client) TimestreamWrite() *timestreamwrite.Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	return timestreamwrite.NewFromConfig(c.cfg)
}

// Transfer returns a client for AWS Transfer service
func (c *Client) Transfer() *transfer.Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	return transfer.NewFromConfig(c.cfg)
}

// WAF returns a client for AWS WAF service
func (c *Client) WAF() *waf.Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	return waf.NewFromConfig(c.cfg)
}

// WAFRegional returns a client for AWS WAFRegional service
func (c *Client) WAFRegional() *wafregional.Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	return wafregional.NewFromConfig(c.cfg)
}

// WAFv2 returns a client for AWS WAFv2 service
func (c *Client) WAFv2() *wafv2.Client {
	c.mu.Lock()
	defer c.mu.Unlock()

	return wafv2.NewFromConfig(c.cfg)
}
