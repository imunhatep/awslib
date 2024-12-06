package main

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	cc "github.com/aws/aws-sdk-go-v2/service/cloudcontrol"
	"github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/provider"
	ptypes "github.com/imunhatep/awslib/provider/types"
	v2 "github.com/imunhatep/awslib/provider/v2"
	"github.com/imunhatep/awslib/service/cloudcontrol"
	"github.com/imunhatep/gocollection/dict"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"os"
	"strconv"
	"time"
)

func main() {
	verbosity, _ := strconv.Atoi(getEnv("LOG_LEVEL", "5"))
	setLogLevel(verbosity)

	awsRegions := []ptypes.AwsRegion{"eu-central-1"}

	providers, err := v2.DefaultAwsClientProviders()
	if err != nil {
		fin(err)
	}

	ctx := context.Background()
	localClientPool := provider.NewClientPool(ctx, v2.NewClientBuilder(ctx, providers...))

	clients, err := localClientPool.GetClients(awsRegions...)
	if err != nil {
		fin(err)
	}

	for _, awsClient := range clients {
		ccRepo := cloudcontrol.NewCloudControlRepository(ctx, awsClient)

		query := &cc.ListResourcesInput{
			TypeName:   aws.String(string(types.ResourceTypeInstance)),
			MaxResults: aws.Int32(2),
		}

		instances, err := ccRepo.ListInstancesByInput(query)
		if err != nil {
			fin(err)
		}

		log.Info().
			Int("resources", len(instances)).
			Msg("found AWS resources")

		for _, resource := range instances {
			fmt.Println("------------------------------------------------")
			log.Info().
				Str("ID", resource.GetIdOrArn()).
				Strs("attrs", dict.Keys(resource.GetAttributes())).
				Strs("tags", dict.Keys(resource.GetTags())).
				Msg("Resource")
		}
	}
}

func fin(err error) {
	fmt.Printf("reason: %s\n", err.Error())
	fmt.Println("trace:")
	fmt.Println(err.(*errors.Error).ErrorStack())
	os.Exit(1)
}

func setLogLevel(level int) {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.DateTime})

	switch level {
	case 0:
		zerolog.SetGlobalLevel(zerolog.FatalLevel)
	case 1:
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	case 2:
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case 3:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case 4:
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.TraceLevel)
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}

	return fallback
}
