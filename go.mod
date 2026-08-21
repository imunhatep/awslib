module github.com/imunhatep/awslib

go 1.25.5

require (
	github.com/Masterminds/semver v1.5.0
	github.com/allegro/bigcache/v3 v3.2.0
	github.com/aws/aws-sdk-go-v2 v1.43.7
	github.com/aws/aws-sdk-go-v2/config v1.32.38
	github.com/aws/aws-sdk-go-v2/credentials v1.19.37
	github.com/aws/aws-sdk-go-v2/service/accessanalyzer v1.51.7
	github.com/aws/aws-sdk-go-v2/service/acm v1.44.2
	github.com/aws/aws-sdk-go-v2/service/apigateway v1.42.7
	github.com/aws/aws-sdk-go-v2/service/athena v1.60.7
	github.com/aws/aws-sdk-go-v2/service/autoscaling v1.72.2
	github.com/aws/aws-sdk-go-v2/service/batch v1.70.0
	github.com/aws/aws-sdk-go-v2/service/cloudcontrol v1.32.7
	github.com/aws/aws-sdk-go-v2/service/cloudformation v1.76.4
	github.com/aws/aws-sdk-go-v2/service/cloudfront v1.68.0
	github.com/aws/aws-sdk-go-v2/service/cloudtrail v1.58.7
	github.com/aws/aws-sdk-go-v2/service/cloudwatch v1.66.6
	github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs v1.82.3
	github.com/aws/aws-sdk-go-v2/service/configservice v1.68.7
	github.com/aws/aws-sdk-go-v2/service/costexplorer v1.67.7
	github.com/aws/aws-sdk-go-v2/service/dynamodb v1.63.4
	github.com/aws/aws-sdk-go-v2/service/ec2 v1.322.0
	github.com/aws/aws-sdk-go-v2/service/ecs v1.90.3
	github.com/aws/aws-sdk-go-v2/service/efs v1.44.7
	github.com/aws/aws-sdk-go-v2/service/eks v1.92.1
	github.com/aws/aws-sdk-go-v2/service/elasticache v1.56.7
	github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2 v1.58.8
	github.com/aws/aws-sdk-go-v2/service/emr v1.64.7
	github.com/aws/aws-sdk-go-v2/service/emrserverless v1.44.7
	github.com/aws/aws-sdk-go-v2/service/glue v1.153.1
	github.com/aws/aws-sdk-go-v2/service/health v1.40.7
	github.com/aws/aws-sdk-go-v2/service/iam v1.59.2
	github.com/aws/aws-sdk-go-v2/service/lambda v1.102.0
	github.com/aws/aws-sdk-go-v2/service/pricing v1.44.7
	github.com/aws/aws-sdk-go-v2/service/rds v1.124.4
	github.com/aws/aws-sdk-go-v2/service/route53 v1.65.9
	github.com/aws/aws-sdk-go-v2/service/route53domains v1.39.2
	github.com/aws/aws-sdk-go-v2/service/s3 v1.107.3
	github.com/aws/aws-sdk-go-v2/service/s3control v1.73.7
	github.com/aws/aws-sdk-go-v2/service/s3outposts v1.37.7
	github.com/aws/aws-sdk-go-v2/service/savingsplans v1.35.7
	github.com/aws/aws-sdk-go-v2/service/secretsmanager v1.44.7
	github.com/aws/aws-sdk-go-v2/service/securityhub v1.76.3
	github.com/aws/aws-sdk-go-v2/service/servicecatalog v1.42.7
	github.com/aws/aws-sdk-go-v2/service/servicediscovery v1.43.7
	github.com/aws/aws-sdk-go-v2/service/servicequotas v1.37.7
	github.com/aws/aws-sdk-go-v2/service/ses v1.37.7
	github.com/aws/aws-sdk-go-v2/service/sfn v1.45.7
	github.com/aws/aws-sdk-go-v2/service/shield v1.37.7
	github.com/aws/aws-sdk-go-v2/service/signer v1.35.7
	github.com/aws/aws-sdk-go-v2/service/sns v1.42.7
	github.com/aws/aws-sdk-go-v2/service/sqs v1.46.7
	github.com/aws/aws-sdk-go-v2/service/ssm v1.73.7
	github.com/aws/aws-sdk-go-v2/service/storagegateway v1.46.7
	github.com/aws/aws-sdk-go-v2/service/sts v1.45.7
	github.com/aws/aws-sdk-go-v2/service/swf v1.37.7
	github.com/aws/aws-sdk-go-v2/service/synthetics v1.47.7
	github.com/aws/aws-sdk-go-v2/service/timestreamwrite v1.38.7
	github.com/aws/aws-sdk-go-v2/service/transfer v1.75.7
	github.com/aws/aws-sdk-go-v2/service/waf v1.33.7
	github.com/aws/aws-sdk-go-v2/service/wafregional v1.33.7
	github.com/aws/aws-sdk-go-v2/service/wafv2 v1.77.6
	github.com/aws/smithy-go v1.27.8
	github.com/go-errors/errors v1.5.1
	github.com/imunhatep/gocollection v0.2.1
	github.com/prometheus/client_golang v1.24.1
	github.com/rs/zerolog v1.35.1
	github.com/stretchr/testify v1.12.1
	golang.org/x/sys v0.47.0
)

require (
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.7.18 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.18.38 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.38 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.38 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.4.39 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.17 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.9.31 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/endpoint-discovery v1.12.15 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.13.38 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.19.39 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.5.7 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.33.7 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.38.7 // indirect
	github.com/barweiss/go-tuple v1.0.2 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.70.1 // indirect
	github.com/prometheus/procfs v0.21.1 // indirect
	github.com/samber/mo v1.7.0 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/exp v0.0.0-20230108222341-4b8118a2686a // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)
