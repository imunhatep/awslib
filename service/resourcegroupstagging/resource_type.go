package resourcegroupstagging

import (
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	ccfg "github.com/imunhatep/awslib/service/cfg"
)

// resourceTypeFilters maps a CloudFormation-style resource type to the
// ResourceTypeFilters value GetResources expects.
//
// AWS provides no conversion between the two, and there is no rule to derive
// one: the API takes "service[:resourceType]" spelled exactly as it appears in
// the resource's ARN, which agrees with the CloudFormation type only by
// coincidence. AWS::EC2::Instance is "ec2:instance", but AWS::RDS::DBInstance is
// "rds:db", AWS::Logs::LogGroup is "logs:log-group", AWS::EFS::FileSystem is
// "elasticfilesystem:file-system" and AWS::EMR::Cluster is
// "elasticmapreduce:cluster". So the table is written out, and it lives here
// rather than in service/cfg because it describes this API's vocabulary, not the
// library's resource types.
//
// Some entries are the bare service name. That is not laziness: the API accepts
// "service" on its own to mean every resource of that service, and it is the
// correct answer where the ARN carries no resource-type segment at all —
// arn:aws:s3:::bucket, arn:aws:sns:region:account:topic and
// arn:aws:sqs:region:account:queue name the resource directly. Guessing
// "s3:bucket" for those risks an InvalidParameterException, and since each of
// those services has effectively one taggable type, the service filter is just
// as narrow.
//
// A type that is absent is absent on purpose — see ResourceTypeFilter.
var resourceTypeFilters = map[cfg.ResourceType]string{
	// athena
	cfg.ResourceTypeAthenaDataCatalog: "athena:datacatalog",
	cfg.ResourceTypeAthenaWorkGroup:   "athena:workgroup",
	// autoscaling — the ARN segment really is camelCase
	cfg.ResourceTypeAutoScalingGroup: "autoscaling:autoScalingGroup",
	// batch
	cfg.ResourceTypeBatchComputeEnvironment: "batch:compute-environment",
	cfg.ResourceTypeBatchJobQueue:           "batch:job-queue",
	// cloudtrail
	cfg.ResourceTypeTrail: "cloudtrail:trail",
	// cloudwatch logs
	ccfg.ResourceTypeCloudWatchLogGroup: "logs:log-group",
	// dynamodb
	cfg.ResourceTypeTable: "dynamodb:table",
	// ec2
	cfg.ResourceTypeInstance:      "ec2:instance",
	cfg.ResourceTypeVolume:        "ec2:volume",
	ccfg.ResourceTypeSnapshot:     "ec2:snapshot",
	cfg.ResourceTypeVpc:           "ec2:vpc",
	cfg.ResourceTypeSubnet:        "ec2:subnet",
	cfg.ResourceTypeSecurityGroup: "ec2:security-group",
	cfg.ResourceTypeVPCEndpoint:   "ec2:vpc-endpoint",
	cfg.ResourceTypeRouteTable:    "ec2:route-table",
	cfg.ResourceTypeEip:           "ec2:elastic-ip",
	// ecs
	cfg.ResourceTypeECSCluster: "ecs:cluster",
	cfg.ResourceTypeECSService: "ecs:service",
	// efs
	cfg.ResourceTypeEFSFileSystem: "elasticfilesystem:file-system",
	// eks
	cfg.ResourceTypeEKSCluster: "eks:cluster",
	// elb
	cfg.ResourceTypeLoadBalancerV2: "elasticloadbalancing:loadbalancer",
	// emr
	ccfg.ResourceTypeEmrCluster: "elasticmapreduce:cluster",
	// emr serverless — ARNs read ".../applications/<id>" with a leading slash and
	// no type segment, so the service filter is the safe spelling
	ccfg.ResourceTypeEmrServerlessApplication: "emr-serverless",
	ccfg.ResourceTypeEmrServerlessJobRun:      "emr-serverless",
	// glue
	ccfg.ResourceTypeGlueDatabase: "glue:database",
	cfg.ResourceTypeGlueJob:       "glue:job",
	ccfg.ResourceTypeGlueTable:    "glue:table",
	// iam
	cfg.ResourceTypeUser:   "iam:user",
	cfg.ResourceTypeRole:   "iam:role",
	cfg.ResourceTypePolicy: "iam:policy",
	// lambda
	cfg.ResourceTypeFunction: "lambda:function",
	// rds
	cfg.ResourceTypeDBInstance: "rds:db",
	cfg.ResourceTypeDBSnapshot: "rds:snapshot",
	// route53
	cfg.ResourceTypeRoute53HostedZone: "route53:hostedzone",
	// s3 — arn:aws:s3:::<bucket> has no resource-type segment
	cfg.ResourceTypeBucket: "s3",
	// secrets manager
	cfg.ResourceTypeSecret: "secretsmanager:secret",
	// sns — arn:aws:sns:<region>:<account>:<topic>
	cfg.ResourceTypeTopic: "sns",
	// sqs — arn:aws:sqs:<region>:<account>:<queue>
	cfg.ResourceTypeQueue: "sqs",
}

// ResourceTypeFilter returns the GetResources filter for a resource type, and
// whether one is known.
//
// The second return value is the point of this function. A wrong filter is worse
// than no filter: GetResources answers an unrecognized one with an empty result
// or an InvalidParameterException, either of which reads as "this resource type
// has no tags". So types whose filter spelling is not established are left out
// of the table — Route53 records and Route53Domains domains, which are not
// ARN-addressable through this API at all; the CloudFront SaaS Manager types;
// and the pseudo-types (CloudTrail events, Health events, Cost Explorer results)
// that are not taggable resources in the first place. Callers fall back to an
// unfiltered sweep, which costs more pages but cannot silently under-report.
func ResourceTypeFilter(resourceType cfg.ResourceType) (string, bool) {
	filter, ok := resourceTypeFilters[resourceType]

	return filter, ok
}

// ResourceTypeFilters converts resource types to GetResources filters, returning
// the filters it knows and the types it does not.
//
// Both halves are returned rather than the caller being handed a quietly
// shortened filter list, because dropping an unmapped type turns a request about
// five types into an answer about four with nothing to show that it happened.
// Duplicate filters are collapsed, which is normal: several EMR Serverless types
// share one.
func ResourceTypeFilters(resourceTypes []cfg.ResourceType) (filters []string, unmapped []cfg.ResourceType) {
	seen := map[string]bool{}

	for _, resourceType := range resourceTypes {
		filter, ok := ResourceTypeFilter(resourceType)
		if !ok {
			unmapped = append(unmapped, resourceType)

			continue
		}

		if seen[filter] {
			continue
		}

		seen[filter] = true
		filters = append(filters, filter)
	}

	return filters, unmapped
}
