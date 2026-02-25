// Example usage of the v3 AWS client with options pattern
package main

import (
	"context"
	"fmt"
	"log"

	v3 "github.com/imunhatep/awslib/provider/v3"

	// Import only the services you need
	"github.com/imunhatep/awslib/provider/v3/clients/ec2"
	"github.com/imunhatep/awslib/provider/v3/clients/s3"
)

func main() {
	ctx := context.Background()

	// Example 1: Create client with specific services
	client, err := v3.NewClient(ctx,
		ec2.WithEC2(),
		s3.WithS3(),
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Account ID: %s\n", client.GetAccountID())
	fmt.Printf("Region: %s\n", client.GetRegion())
	fmt.Printf("Enabled services: %v\n", client.EnabledServices())

	// Example 2: Use EC2 service
	ec2Client, err := ec2.GetClient(client)
	if err != nil {
		log.Fatal(err)
	}

	regions, err := ec2Client.DescribeRegions(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Found %d EC2 regions\n", len(regions.Regions))

	// Example 3: Use S3 service
	s3Client, err := s3.GetClient(client)
	if err != nil {
		log.Fatal(err)
	}

	buckets, err := s3Client.ListBuckets(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Found %d S3 buckets\n", len(buckets.Buckets))

	// Example 4: Check if service is enabled
	if client.IsServiceEnabled("dynamodb") {
		fmt.Println("DynamoDB is enabled")
	} else {
		fmt.Println("DynamoDB is not enabled")
	}
}
