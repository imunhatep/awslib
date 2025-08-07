package awslib

import (
	"encoding/gob"
	typesEMR "github.com/aws/aws-sdk-go-v2/service/emrserverless/types"
	"github.com/imunhatep/awslib/service/athena"
	"github.com/imunhatep/awslib/service/autoscaling"
	"github.com/imunhatep/awslib/service/batch"
	"github.com/imunhatep/awslib/service/cloudcontrol"
	"github.com/imunhatep/awslib/service/cloudtrail"
	"github.com/imunhatep/awslib/service/cloudwatchlogs"
	"github.com/imunhatep/awslib/service/dynamodb"
	"github.com/imunhatep/awslib/service/ec2"
	"github.com/imunhatep/awslib/service/ecs"
	"github.com/imunhatep/awslib/service/efs"
	"github.com/imunhatep/awslib/service/eks"
	"github.com/imunhatep/awslib/service/elb"
	"github.com/imunhatep/awslib/service/emr"
	"github.com/imunhatep/awslib/service/emrserverless"
	"github.com/imunhatep/awslib/service/glue"
	"github.com/imunhatep/awslib/service/iam"
	"github.com/imunhatep/awslib/service/lambda"
	"github.com/imunhatep/awslib/service/rds"
	"github.com/imunhatep/awslib/service/route53"
	"github.com/imunhatep/awslib/service/s3"
	"github.com/imunhatep/awslib/service/secretmanager"
	"github.com/imunhatep/awslib/service/sns"
	"github.com/imunhatep/awslib/service/sqs"
	"github.com/rs/zerolog/log"
	"go/importer"
	"go/types"
	"reflect"
)

var awsServicesGobRegistered = false

func init() {
	GobRegisterAwsServices()
}

func GobRegisterAwsServices() {
	if awsServicesGobRegistered == true {
		return
	}

	// AutoScaling
	gob.Register(autoscaling.AutoScalingGroup{})
	gob.Register(autoscaling.AutoScalingGroupList{})

	// Athena
	gob.Register(athena.DataCatalog{})
	gob.Register(athena.WorkGroup{})
	gob.Register(athena.WorkGroupList{})

	// Batch
	gob.Register(batch.ComputeEnvironment{})
	gob.Register(batch.ComputeEnvironmentList{})
	gob.Register(batch.JobQueue{})

	// CloudControl
	gob.Register(cloudcontrol.Bucket{})
	gob.Register(cloudcontrol.Instance{})
	gob.Register(cloudcontrol.Volume{})

	// CloudTrail
	gob.Register(cloudtrail.CloudTrailEvent{})

	// Cloudwatchlogs
	gob.Register(cloudwatchlogs.LogGroup{})

	// DynamoDB
	gob.Register(dynamodb.Table{})

	// ec2
	gob.Register(ec2.Instance{})
	gob.Register(ec2.Snapshot{})
	gob.Register(ec2.Volume{})
	gob.Register(ec2.Vpc{})

	// ECS
	gob.Register(ecs.Cluster{})
	gob.Register(ecs.Service{})

	// EFS
	gob.Register(efs.FileSystem{})
	gob.Register(efs.FileSystemList{})

	// EKS
	gob.Register(eks.Cluster{})
	gob.Register(eks.ClusterList{})

	// ELB
	gob.Register(elb.LoadBalancer{})
	gob.Register(elb.LoadBalancerList{})

	// EMR
	gob.Register(emr.Cluster{})
	gob.Register(emrserverless.Application{})
	gob.Register(emrserverless.JobRun{})

	gob.Register(typesEMR.JobDriverMemberSparkSubmit{})
	gob.Register(typesEMR.JobDriverMemberHive{})

	// Glue
	gob.Register(glue.Database{})
	gob.Register(glue.Job{})
	gob.Register(glue.Table{})

	// IAM
	gob.Register(iam.Policy{})
	gob.Register(iam.PolicyVersion{})
	gob.Register(iam.PolicyDocument{})
	gob.Register(iam.Role{})
	gob.Register(iam.Statement{})
	gob.Register(iam.User{})

	// Lambda
	gob.Register(lambda.Function{})

	// Health
	//gob.Register(health.Event{})

	// RDS
	gob.Register(rds.DbInstance{})
	gob.Register(rds.DbInstanceList{})
	gob.Register(rds.DbSnapshot{})

	// Route53
	gob.Register(route53.HostedZone{})
	gob.Register(route53.HostedZoneList{})

	// Secret Manager
	gob.Register(secretmanager.SecretEntry{})
	gob.Register(secretmanager.SecretValue{})

	// S3
	gob.Register(s3.Bucket{})

	// SNS
	gob.Register(sns.Topic{})

	// SQS
	gob.Register(sqs.Queue{})

	awsServicesGobRegistered = true
}

func GobRegisterAwsServicesAll() {
	if awsServicesGobRegistered == true {
		return
	}

	// Load the "service" package
	pkg, err := importer.Default().Import("github.com/imunhatep/awslib/service")
	if err != nil {
		log.Error().Err(err).Msg("[GOB] failed to import package for registering")
		return
	}

	// Iterate over all package members
	scope := pkg.Scope()
	for _, name := range scope.Names() {
		obj := scope.Lookup(name)
		if obj, ok := obj.(*types.TypeName); ok {
			typ := obj.Type()
			if structType, ok := typ.Underlying().(*types.Struct); ok {
				// Register the struct with gob
				registerStruct(name, structType)
			}
		}
	}

	awsServicesGobRegistered = true
}

func registerStruct(name string, structType *types.Struct) {
	// Dynamically create an instance of the struct
	typ := reflect.TypeOf(structType)
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	instance := reflect.New(typ).Interface()

	// Register the struct with gob
	gob.Register(instance)
	log.Trace().Str("struct", name).Msg("[GOB] registered struct")
}
