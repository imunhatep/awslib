module github.com/imunhatep/awslib

go 1.23.2

require (
	github.com/Masterminds/semver v1.5.0
	github.com/allegro/bigcache/v3 v3.1.0
	github.com/aws/aws-sdk-go-v2 v1.32.5
	github.com/aws/aws-sdk-go-v2/config v1.28.5
	github.com/aws/aws-sdk-go-v2/credentials v1.17.46
	github.com/aws/aws-sdk-go-v2/service/accessanalyzer v1.36.1
	github.com/aws/aws-sdk-go-v2/service/acm v1.30.6
	github.com/aws/aws-sdk-go-v2/service/apigateway v1.28.0
	github.com/aws/aws-sdk-go-v2/service/athena v1.48.4
	github.com/aws/aws-sdk-go-v2/service/autoscaling v1.51.0
	github.com/aws/aws-sdk-go-v2/service/batch v1.48.1
	github.com/aws/aws-sdk-go-v2/service/cloudcontrol v1.23.1
	github.com/aws/aws-sdk-go-v2/service/cloudformation v1.56.0
	github.com/aws/aws-sdk-go-v2/service/cloudtrail v1.46.1
	github.com/aws/aws-sdk-go-v2/service/cloudwatch v1.43.1
	github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs v1.44.0
	github.com/aws/aws-sdk-go-v2/service/configservice v1.51.0
	github.com/aws/aws-sdk-go-v2/service/costexplorer v1.45.0
	github.com/aws/aws-sdk-go-v2/service/dynamodb v1.37.1
	github.com/aws/aws-sdk-go-v2/service/ec2 v1.194.0
	github.com/aws/aws-sdk-go-v2/service/ecs v1.52.0
	github.com/aws/aws-sdk-go-v2/service/efs v1.34.0
	github.com/aws/aws-sdk-go-v2/service/eks v1.52.1
	github.com/aws/aws-sdk-go-v2/service/elasticache v1.44.0
	github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2 v1.43.0
	github.com/aws/aws-sdk-go-v2/service/emr v1.47.0
	github.com/aws/aws-sdk-go-v2/service/emrserverless v1.26.6
	github.com/aws/aws-sdk-go-v2/service/glue v1.102.0
	github.com/aws/aws-sdk-go-v2/service/health v1.29.0
	github.com/aws/aws-sdk-go-v2/service/iam v1.38.1
	github.com/aws/aws-sdk-go-v2/service/lambda v1.69.0
	github.com/aws/aws-sdk-go-v2/service/pricing v1.32.6
	github.com/aws/aws-sdk-go-v2/service/rds v1.91.0
	github.com/aws/aws-sdk-go-v2/service/route53 v1.46.2
	github.com/aws/aws-sdk-go-v2/service/s3 v1.69.0
	github.com/aws/aws-sdk-go-v2/service/s3control v1.50.1
	github.com/aws/aws-sdk-go-v2/service/s3outposts v1.28.6
	github.com/aws/aws-sdk-go-v2/service/savingsplans v1.23.6
	github.com/aws/aws-sdk-go-v2/service/secretsmanager v1.34.6
	github.com/aws/aws-sdk-go-v2/service/securityhub v1.54.7
	github.com/aws/aws-sdk-go-v2/service/servicecatalog v1.32.6
	github.com/aws/aws-sdk-go-v2/service/servicediscovery v1.33.6
	github.com/aws/aws-sdk-go-v2/service/servicequotas v1.25.6
	github.com/aws/aws-sdk-go-v2/service/ses v1.29.0
	github.com/aws/aws-sdk-go-v2/service/sfn v1.34.0
	github.com/aws/aws-sdk-go-v2/service/shield v1.29.6
	github.com/aws/aws-sdk-go-v2/service/signer v1.26.6
	github.com/aws/aws-sdk-go-v2/service/sns v1.33.6
	github.com/aws/aws-sdk-go-v2/service/sqs v1.37.1
	github.com/aws/aws-sdk-go-v2/service/ssm v1.56.0
	github.com/aws/aws-sdk-go-v2/service/storagegateway v1.34.6
	github.com/aws/aws-sdk-go-v2/service/sts v1.33.1
	github.com/aws/aws-sdk-go-v2/service/swf v1.27.7
	github.com/aws/aws-sdk-go-v2/service/synthetics v1.30.1
	github.com/aws/aws-sdk-go-v2/service/timestreamwrite v1.29.7
	github.com/aws/aws-sdk-go-v2/service/transfer v1.53.5
	github.com/aws/aws-sdk-go-v2/service/waf v1.25.6
	github.com/aws/aws-sdk-go-v2/service/wafregional v1.25.6
	github.com/aws/aws-sdk-go-v2/service/wafv2 v1.55.5
	github.com/aws/smithy-go v1.22.1
	github.com/go-errors/errors v1.5.1
	github.com/imunhatep/gocollection v0.2.1
	github.com/prometheus/client_golang v1.20.5
	github.com/rs/zerolog v1.33.0
	github.com/stretchr/testify v1.10.0
	golang.org/x/sys v0.27.0
)

require (
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.6.7 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.16.20 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.3.24 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.6.24 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.1 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.3.24 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.12.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.4.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/endpoint-discovery v1.10.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.12.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.18.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.24.6 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.28.5 // indirect
	github.com/barweiss/go-tuple v1.0.2 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.19 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common v0.55.0 // indirect
	github.com/prometheus/procfs v0.15.1 // indirect
	github.com/samber/mo v1.7.0 // indirect
	golang.org/x/exp v0.0.0-20230108222341-4b8118a2686a // indirect
	google.golang.org/protobuf v1.34.2 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
