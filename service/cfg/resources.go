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
		awscfg.ResourceTypeUser,
		// route53
		awscfg.ResourceTypeRoute53HostedZone,
		ResourceTypeRoute53ResourceRecord,
		ResourceTypeRoute53Domain,
		ResourceTypeRoute53DomainSummary,
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
		awscfg.ResourceTypeVpc,
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
