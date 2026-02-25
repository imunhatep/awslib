// Example: Using all services (compatibility mode)
package main

import (
	"context"
	"fmt"
	"log"

	v3 "github.com/imunhatep/awslib/provider/v3"

	// Import the "all" package to enable all services
	// WARNING: This defeats the purpose of selective compilation
	// Only use for testing or when you truly need all services
	"github.com/imunhatep/awslib/provider/v3/clients/all"

	// Import service helpers
	"github.com/imunhatep/awslib/provider/v3/clients/ec2"
	"github.com/imunhatep/awslib/provider/v3/clients/rds"
	"github.com/imunhatep/awslib/provider/v3/clients/s3"
)

func main() {
	ctx := context.Background()

	// Enable all AWS services at once
	// This is equivalent to the old v2 client behavior
	client, err := v3.NewClient(ctx, all.WithAll())
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Total enabled services: %d\n", len(client.EnabledServices()))

	// All services are now available
	ec2Client := ec2.MustGetClient(client)
	s3Client := s3.MustGetClient(client)
	rdsClient := rds.MustGetClient(client)

	// Use the services
	regions, _ := ec2Client.DescribeRegions(ctx, nil)
	fmt.Printf("EC2 regions: %d\n", len(regions.Regions))

	buckets, _ := s3Client.ListBuckets(ctx, nil)
	fmt.Printf("S3 buckets: %d\n", len(buckets.Buckets))

	databases, _ := rdsClient.DescribeDBInstances(ctx, nil)
	fmt.Printf("RDS databases: %d\n", len(databases.DBInstances))
}
