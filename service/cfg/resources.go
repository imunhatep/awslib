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

	// CloudFront SaaS Manager (multi-tenant distributions). ListDistributionTenants
	// returns summaries; the full tenant only comes back from a Get, so the two are
	// tracked as distinct resource types.
	ResourceTypeCloudFrontDistributionTenant        awscfg.ResourceType = "AWS::CloudFront::DistributionTenant"
	ResourceTypeCloudFrontDistributionTenantSummary awscfg.ResourceType = "AWS::CloudFront::DistributionTenantSummary"
	ResourceTypeCloudFrontConnectionGroup           awscfg.ResourceType = "AWS::CloudFront::ConnectionGroup"
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
	}
}

func ResourceTypeListRegional() []awscfg.ResourceType {
	return []awscfg.ResourceType{
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
		// elb
		awscfg.ResourceTypeLoadBalancerV2,
		// sqs
		awscfg.ResourceTypeQueue,
		// sm
		awscfg.ResourceTypeSecret,
		// sns
		awscfg.ResourceTypeTable,
		awscfg.ResourceTypeTopic,
	}
}
