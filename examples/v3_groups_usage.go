// Example usage of v3 client with service groups
//go:build ignore

package main

import (
	"context"
	"fmt"
	"log"

	v3 "github.com/imunhatep/awslib/provider/v3"

	// Import individual service helpers for typed access
	"github.com/imunhatep/awslib/provider/v3/clients/ec2"
	"github.com/imunhatep/awslib/provider/v3/clients/lambda"
	"github.com/imunhatep/awslib/provider/v3/clients/s3"
)

func main() {
	ctx := context.Background()

	// Create client with service groups
	client, err := v3.NewClient(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// Use services from the compute group
	ec2Client := ec2.GetClient(client)
	regions, err := ec2Client.DescribeRegions(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("EC2 regions: %d\n", len(regions.Regions))

	lambdaClient := lambda.GetClient(client)
	functions, err := lambdaClient.ListFunctions(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Lambda functions: %d\n", len(functions.Functions))

	// Use services from the storage group
	s3Client := s3.GetClient(client)
	buckets, err := s3Client.ListBuckets(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("S3 buckets: %d\n", len(buckets.Buckets))
}
