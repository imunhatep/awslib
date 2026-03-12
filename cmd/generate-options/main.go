// Code generator for AWS service clients
//go:build ignore

package main

import (
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// ServiceInfo holds information about an AWS service
type ServiceInfo struct {
	Name           string // e.g., "ec2", "s3", "dynamodb"
	PackageName    string // e.g., "ec2", "s3", "dynamodb"
	ImportPath     string // e.g., "github.com/aws/aws-sdk-go-v2/service/ec2"
	ServiceName    string // e.g., "EC2", "S3", "DynamoDB"
	FunctionPrefix string // e.g., "WithEC2", "WithS3", "WithDynamoDB"
}

// Template for service option
const optionTemplate = `// Package {{.PackageName}} provides {{.ServiceName}} service access for v3 client
// This file is auto-generated. DO NOT EDIT.
package {{.PackageName}}

import (
	"{{.ImportPath}}"
	v3 "github.com/imunhatep/awslib/provider/v3"
)

const serviceName = "{{.Name}}"

// GetClient returns a cached or new {{.ServiceName}} client
func GetClient(client *v3.Client, optFns ...func(*{{.PackageName}}.Options)) *{{.PackageName}}.Client {
	// Check cache first
	if cached, ok := client.GetCachedService(serviceName); ok {
		return cached.(*{{.PackageName}}.Client)
	}

	// Create new client
	svc := {{.PackageName}}.NewFromConfig(client.Config(), optFns...)

	// Cache it
	client.CacheService(serviceName, svc)

	return svc
}
`

// titleCase converts strings like "ec2" to "EC2", "dynamodb" to "DynamoDB"
func titleCase(s string) string {
	// Special cases for acronyms
	acronyms := map[string]string{
		"ec2":                    "EC2",
		"s3":                     "S3",
		"s3control":              "S3Control",
		"s3outposts":             "S3Outposts",
		"rds":                    "RDS",
		"iam":                    "IAM",
		"sns":                    "SNS",
		"sqs":                    "SQS",
		"acm":                    "ACM",
		"eks":                    "EKS",
		"ecs":                    "ECS",
		"efs":                    "EFS",
		"emr":                    "EMR",
		"ssm":                    "SSM",
		"sts":                    "STS",
		"waf":                    "WAF",
		"wafv2":                  "WAFv2",
		"wafregional":            "WAFRegional",
		"dynamodb":               "DynamoDB",
		"cloudwatch":             "CloudWatch",
		"cloudwatchlogs":         "CloudWatchLogs",
		"cloudtrail":             "CloudTrail",
		"cloudformation":         "CloudFormation",
		"cloudcontrol":           "CloudControl",
		"apigateway":             "APIGateway",
		"elasticache":            "ElastiCache",
		"elasticloadbalancingv2": "ElasticLoadBalancingV2",
		"emrserverless":          "EMRServerless",
		"glue":                   "Glue",
		"lambda":                 "Lambda",
		"athena":                 "Athena",
		"autoscaling":            "AutoScaling",
		"batch":                  "Batch",
		"costexplorer":           "CostExplorer",
		"health":                 "Health",
		"pricing":                "Pricing",
		"route53":                "Route53",
		"secretsmanager":         "SecretsManager",
		"securityhub":            "SecurityHub",
		"servicecatalog":         "ServiceCatalog",
		"servicediscovery":       "ServiceDiscovery",
		"servicequotas":          "ServiceQuotas",
		"ses":                    "SES",
		"sfn":                    "StepFunctions",
		"shield":                 "Shield",
		"signer":                 "Signer",
		"storagegateway":         "StorageGateway",
		"swf":                    "SWF",
		"synthetics":             "Synthetics",
		"timestreamwrite":        "TimestreamWrite",
		"transfer":               "Transfer",
		"accessanalyzer":         "AccessAnalyzer",
		"savingsplans":           "SavingsPlans",
		"configservice":          "ConfigService",
	}

	if title, ok := acronyms[s]; ok {
		return title
	}

	// Default: capitalize first letter
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// List of AWS services to generate
var awsServices = []string{
	"accessanalyzer",
	"acm",
	"apigateway",
	"athena",
	"autoscaling",
	"batch",
	"cloudcontrol",
	"cloudformation",
	"cloudtrail",
	"cloudwatch",
	"cloudwatchlogs",
	"configservice",
	"costexplorer",
	"dynamodb",
	"ec2",
	"ecs",
	"efs",
	"eks",
	"elasticache",
	"elasticloadbalancingv2",
	"emr",
	"emrserverless",
	"glue",
	"health",
	"iam",
	"lambda",
	"pricing",
	"rds",
	"route53",
	"s3",
	"s3control",
	"s3outposts",
	"savingsplans",
	"secretsmanager",
	"securityhub",
	"servicecatalog",
	"servicediscovery",
	"servicequotas",
	"ses",
	"sfn",
	"shield",
	"signer",
	"sns",
	"sqs",
	"ssm",
	"storagegateway",
	"swf",
	"synthetics",
	"timestreamwrite",
	"transfer",
	"waf",
	"wafregional",
	"wafv2",
}

func main() {
	tmpl, err := template.New("client").Parse(optionTemplate)
	if err != nil {
		panic(err)
	}

	baseDir := "provider/v3/clients"
	os.MkdirAll(baseDir, 0755)

	generated := 0
	for _, serviceName := range awsServices {
		service := ServiceInfo{
			Name:           serviceName,
			PackageName:    serviceName,
			ImportPath:     fmt.Sprintf("github.com/aws/aws-sdk-go-v2/service/%s", serviceName),
			ServiceName:    titleCase(serviceName),
			FunctionPrefix: "With" + titleCase(serviceName),
		}

		dir := filepath.Join(baseDir, serviceName)
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			panic(err)
		}

		filename := filepath.Join(dir, serviceName+".go")
		file, err := os.Create(filename)
		if err != nil {
			panic(err)
		}

		err = tmpl.Execute(file, service)
		if err != nil {
			file.Close()
			panic(err)
		}
		file.Close()

		// Format the generated code
		content, err := os.ReadFile(filename)
		if err != nil {
			panic(err)
		}

		formatted, err := format.Source(content)
		if err != nil {
			fmt.Printf("Warning: failed to format %s: %v\n", filename, err)
			continue
		}

		err = os.WriteFile(filename, formatted, 0644)
		if err != nil {
			panic(err)
		}

		generated++
	}

	fmt.Printf("✓ Generated %d service option packages in %s\n", generated, baseDir)
}
