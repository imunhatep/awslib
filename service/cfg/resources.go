package cfg

import (
	"strings"

	awscfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/imunhatep/gocollection/slice"
)

const (
	ResourceTypeDBEngineVersion          awscfg.ResourceType = "AWS::RDS::DBEngineVersion"
	ResourceTypeSnapshot                 awscfg.ResourceType = "AWS::EC2::Snapshot"
	ResourceTypeEmrServerlessApplication awscfg.ResourceType = "AWS::EMRServerless::Application"
	ResourceTypeEmrCluster               awscfg.ResourceType = "AWS::EMR::Cluster"
	ResourceTypeEmrServerlessJobRun      awscfg.ResourceType = "AWS::EMRServerless::JobRun"
	ResourceTypeCloudWatchLogGroup       awscfg.ResourceType = "AWS::Logs::LogGroup"
	ResourceTypeGlueDatabase             awscfg.ResourceType = "AWS::Glue::Database"
	ResourceTypeGlueTable                awscfg.ResourceType = "AWS::Glue::Table"
	ResourceTypeGlueJob                  awscfg.ResourceType = "AWS::Glue::Job"
	ResourceTypeTrailEvent               awscfg.ResourceType = "AWS::CloudTrail::Event"
	ResourceTypeHealthEvent              awscfg.ResourceType = "AWS::Health::Event"
	ResourceTypeRoute53ResourceRecord    awscfg.ResourceType = "AWS::Route53::ResourceRecord"
	ResourceTypeRoute53DomainSummary     awscfg.ResourceType = "AWS::Route53Domains::DomainSummary"
	ResourceTypeRoute53Domain            awscfg.ResourceType = "AWS::Route53Domains::Domain"
	ResourceTypeCostAndUsage             awscfg.ResourceType = "AWS::CostExplorer::CostAndUsage"
	ResourceTypeCostForecast             awscfg.ResourceType = "AWS::CostExplorer::CostForecast"
	ResourceTypeCostDimensionValue       awscfg.ResourceType = "AWS::CostExplorer::DimensionValue"

	// Cost Anomaly Detection. Like the Cost Explorer types above, these only label
	// metrics: an anomaly is a finding about spend, and monitors and subscriptions
	// configure how findings are made and reported. All three are deliberately
	// absent from ResourceTypeList and from proxy.RepoProxy.FindAll.
	ResourceTypeCostAnomaly             awscfg.ResourceType = "AWS::CostExplorer::Anomaly"
	ResourceTypeCostAnomalyMonitor      awscfg.ResourceType = "AWS::CostExplorer::AnomalyMonitor"
	ResourceTypeCostAnomalySubscription awscfg.ResourceType = "AWS::CostExplorer::AnomalySubscription"

	// Savings Plans calls answer about commitments and the rates they are offered
	// at, not about anything fetchable: a purchased plan is closer to a contract
	// than a resource, and an offering rate is a price. Both are deliberately absent
	// from ResourceTypeList and from proxy.RepoProxy.FindAll — they exist only to
	// label metrics, the same way ResourceTypeTagMapping below does.
	ResourceTypeSavingsPlan             awscfg.ResourceType = "AWS::SavingsPlans::SavingsPlan"
	ResourceTypeSavingsPlanOfferingRate awscfg.ResourceType = "AWS::SavingsPlans::OfferingRate"

	// ResourceTypeTagMapping labels Resource Groups Tagging API calls, which
	// answer about tags across many resource types at once and so have no single
	// type of their own. Deliberately absent from ResourceTypeList: nothing
	// fetches it as a resource.
	ResourceTypeTagMapping awscfg.ResourceType = "AWS::ResourceGroupsTagging::TagMapping"

	// ResourceTypeCloudControlRequest labels the Cloud Control calls that act on a
	// resource request rather than a resource: status polling, cancellation and
	// listing of requests, which carry a RequestToken and no type of their own.
	// Deliberately absent from ResourceTypeList: a request is not fetchable.
	ResourceTypeCloudControlRequest awscfg.ResourceType = "AWS::CloudControl::ResourceRequest"

	// ResourceTypeCloudWatchLogsQuery labels CloudWatch Logs Insights calls
	// (StartQuery, GetQueryResults, StopQuery). A query result is an answer about
	// log events, not a resource: deliberately absent from ResourceTypeList.
	ResourceTypeCloudWatchLogsQuery awscfg.ResourceType = "AWS::Logs::Query"

	// CloudFront SaaS Manager (multi-tenant distributions). ListDistributionTenants
	// returns summaries; the full tenant only comes back from a Get, so the two are
	// tracked as distinct resource types.
	ResourceTypeCloudFrontDistributionTenant        awscfg.ResourceType = "AWS::CloudFront::DistributionTenant"
	ResourceTypeCloudFrontDistributionTenantSummary awscfg.ResourceType = "AWS::CloudFront::DistributionTenantSummary"
	ResourceTypeCloudFrontConnectionGroup           awscfg.ResourceType = "AWS::CloudFront::ConnectionGroup"

	// CloudFront configuration objects a distribution references by ID. AWS Config
	// models only PublicKey, RealtimeLogConfig and KeyValueStore among these (use
	// the SDK constants for those); the rest carry their CloudFormation names.
	ResourceTypeCloudFrontCachePolicy                awscfg.ResourceType = "AWS::CloudFront::CachePolicy"
	ResourceTypeCloudFrontOriginRequestPolicy        awscfg.ResourceType = "AWS::CloudFront::OriginRequestPolicy"
	ResourceTypeCloudFrontResponseHeadersPolicy      awscfg.ResourceType = "AWS::CloudFront::ResponseHeadersPolicy"
	ResourceTypeCloudFrontFunction                   awscfg.ResourceType = "AWS::CloudFront::Function"
	ResourceTypeCloudFrontOriginAccessControl        awscfg.ResourceType = "AWS::CloudFront::OriginAccessControl"
	ResourceTypeCloudFrontOriginAccessIdentity       awscfg.ResourceType = "AWS::CloudFront::CloudFrontOriginAccessIdentity"
	ResourceTypeCloudFrontKeyGroup                   awscfg.ResourceType = "AWS::CloudFront::KeyGroup"
	ResourceTypeCloudFrontVpcOrigin                  awscfg.ResourceType = "AWS::CloudFront::VpcOrigin"
	ResourceTypeCloudFrontContinuousDeploymentPolicy awscfg.ResourceType = "AWS::CloudFront::ContinuousDeploymentPolicy"
	ResourceTypeCloudFrontPublicKey                  awscfg.ResourceType = awscfg.ResourceTypeCloudFrontPublicKey
	ResourceTypeCloudFrontRealtimeLogConfig          awscfg.ResourceType = awscfg.ResourceTypeCloudFrontRealtimeLogConfig
	ResourceTypeCloudFrontKeyValueStore              awscfg.ResourceType = awscfg.ResourceTypeCloudFrontKeyValueStore

	// WAFv2. AWS Config's own types, aliased so callers find them next to the
	// rest. Every WAFv2 object has a scope: REGIONAL ones live in the region that
	// protects an ALB, API Gateway stage, AppSync API, Cognito pool, App Runner
	// service or Verified Access instance; CLOUDFRONT ones are global and served
	// only from us-east-1. The repository lists both scopes from a us-east-1
	// client and REGIONAL only elsewhere, which is why these are regional types:
	// a sweep sees every CloudFront-scoped object exactly once.
	ResourceTypeWafv2WebACL          awscfg.ResourceType = awscfg.ResourceTypeWebACLV2
	ResourceTypeWafv2RuleGroup       awscfg.ResourceType = awscfg.ResourceTypeRuleGroupV2
	ResourceTypeWafv2IPSet           awscfg.ResourceType = awscfg.ResourceTypeIPSetV2
	ResourceTypeWafv2RegexPatternSet awscfg.ResourceType = awscfg.ResourceTypeRegexPatternSetV2

	// Labels for WAFv2 calls that answer about traffic rather than a resource:
	// sampled requests and the resources a web ACL protects. Deliberately absent
	// from ResourceTypeList — nothing fetches them as resources.
	ResourceTypeWafv2SampledRequest    awscfg.ResourceType = "AWS::WAFv2::SampledRequest"
	ResourceTypeWafv2WebACLAssociation awscfg.ResourceType = "AWS::WAFv2::WebACLAssociation"

	// ResourceTypeCloudWatchMetric labels CloudWatch metric reads (ListMetrics,
	// GetMetricData). A metric is a time series, not a resource, so it is absent
	// from ResourceTypeList like the other label-only types.
	ResourceTypeCloudWatchMetric awscfg.ResourceType = "AWS::CloudWatch::Metric"

	// ACM. ListCertificates returns summaries without validation records; the
	// full certificate only comes back from DescribeCertificate, so the two are
	// tracked as distinct resource types. The full one is AWS Config's own type.
	ResourceTypeAcmCertificate        awscfg.ResourceType = awscfg.ResourceTypeCertificate
	ResourceTypeAcmCertificateSummary awscfg.ResourceType = "AWS::ACM::CertificateSummary"
)

func ResourceTypeToString(r awscfg.ResourceType) string {
	return strings.ToLower(string(r))
}

func ResourceTypeToUrl(r awscfg.ResourceType) string {
	return strings.ReplaceAll(ResourceTypeToString(r), "::", "_")
}

func ResourceTypeFromUrl(t string) (awscfg.ResourceType, bool) {
	eType := slice.Find(ResourceTypeList(), func(e awscfg.ResourceType) bool { return ResourceTypeToUrl(e) == t })
	return eType.OrEmpty(), eType.IsPresent()
}

func ResourceTypeSort(s1, s2 awscfg.ResourceType) bool {
	return string(s1) < string(s2)
}

func ResourceTypeList() []awscfg.ResourceType {
	return append(ResourceTypeListRegional(), ResourceTypeListGlobal()...)
}

func ResourceTypeListGlobal() []awscfg.ResourceType {
	return []awscfg.ResourceType{
		// iam — one global endpoint, so enumerating these per region returns the same
		// objects once per region
		awscfg.ResourceTypeUser,
		awscfg.ResourceTypeRole,
		awscfg.ResourceTypePolicy,
		// route53
		awscfg.ResourceTypeRoute53HostedZone,
		ResourceTypeRoute53ResourceRecord,
		ResourceTypeRoute53Domain,
		ResourceTypeRoute53DomainSummary,
		// cloudfront — the control plane is global, not regional
		ResourceTypeCloudFrontDistributionTenantSummary,
		ResourceTypeCloudFrontConnectionGroup,
		awscfg.ResourceTypeDistribution,
		ResourceTypeCloudFrontCachePolicy,
		ResourceTypeCloudFrontOriginRequestPolicy,
		ResourceTypeCloudFrontResponseHeadersPolicy,
		ResourceTypeCloudFrontFunction,
		ResourceTypeCloudFrontOriginAccessControl,
		ResourceTypeCloudFrontOriginAccessIdentity,
		ResourceTypeCloudFrontKeyGroup,
		ResourceTypeCloudFrontPublicKey,
		ResourceTypeCloudFrontRealtimeLogConfig,
		ResourceTypeCloudFrontKeyValueStore,
		ResourceTypeCloudFrontVpcOrigin,
		ResourceTypeCloudFrontContinuousDeploymentPolicy,
	}
}

func ResourceTypeListRegional() []awscfg.ResourceType {
	return []awscfg.ResourceType{
		// acm
		ResourceTypeAcmCertificateSummary,
		// athena
		awscfg.ResourceTypeAthenaDataCatalog,
		awscfg.ResourceTypeAthenaWorkGroup,
		// asg
		awscfg.ResourceTypeAutoScalingGroup,
		// batch
		awscfg.ResourceTypeBatchComputeEnvironment,
		awscfg.ResourceTypeBatchJobQueue,
		// cloudwatch
		ResourceTypeCloudWatchLogGroup,
		// cloudtrail
		awscfg.ResourceTypeTrail,
		// s3 bucket
		awscfg.ResourceTypeBucket,
		// rds
		awscfg.ResourceTypeDBInstance,
		awscfg.ResourceTypeDBSnapshot,
		// ecs
		awscfg.ResourceTypeECSCluster,
		awscfg.ResourceTypeECSService,
		// eks
		awscfg.ResourceTypeEKSCluster,
		// efs
		awscfg.ResourceTypeEFSFileSystem,
		// emr
		ResourceTypeEmrCluster,
		ResourceTypeEmrServerlessApplication,
		ResourceTypeEmrServerlessJobRun,
		// lambda
		awscfg.ResourceTypeFunction,
		// glue
		ResourceTypeGlueDatabase,
		awscfg.ResourceTypeGlueJob,
		ResourceTypeGlueTable,
		// ec2
		awscfg.ResourceTypeInstance,
		awscfg.ResourceTypeVolume,
		ResourceTypeSnapshot,
		// ec2 — vpc and its networking resources
		awscfg.ResourceTypeVpc,
		awscfg.ResourceTypeSubnet,
		awscfg.ResourceTypeSecurityGroup,
		awscfg.ResourceTypeVPCEndpoint,
		awscfg.ResourceTypeRouteTable,
		awscfg.ResourceTypeEip,
		awscfg.ResourceTypeNetworkInterface,
		// elb
		awscfg.ResourceTypeLoadBalancerV2,
		// sqs
		awscfg.ResourceTypeQueue,
		// sm
		awscfg.ResourceTypeSecret,
		// sns
		awscfg.ResourceTypeTable,
		awscfg.ResourceTypeTopic,
		// wafv2 — REGIONAL scope per region, plus CLOUDFRONT scope from us-east-1
		ResourceTypeWafv2WebACL,
		ResourceTypeWafv2RuleGroup,
		ResourceTypeWafv2IPSet,
		ResourceTypeWafv2RegexPatternSet,
	}
}
