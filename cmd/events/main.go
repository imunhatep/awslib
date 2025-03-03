package main

import (
	"context"
	"fmt"
	"github.com/allegro/bigcache/v3"
	"github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/cache"
	"github.com/imunhatep/awslib/cache/handlers"
	"github.com/imunhatep/awslib/provider"
	ptypes "github.com/imunhatep/awslib/provider/types"
	v2 "github.com/imunhatep/awslib/provider/v2"
	"github.com/imunhatep/awslib/service"
	"github.com/imunhatep/awslib/service/cloudtrail"
	"github.com/imunhatep/gocollection/slice"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"os"
	"strconv"
	"strings"
	"time"
)

func main() {
	verbosity, _ := strconv.Atoi(getEnv("LOG_LEVEL", "5"))
	setLogLevel(verbosity)

	err := awsEvents()
	if err != nil {
		fin(err)
	}
}

func awsEvents() error {
	ctx := context.Background()

	awsRegions := []ptypes.AwsRegion{"eu-central-1"}
	eventName := getEnv("EVENT_NAME", "")
	resourceId := getEnv("EVENT_RESOURCE", "")
	resourceType := getEnv("EVENT_TYPE", "")
	username := getEnv("EVENT_USER", "")
	domain := getEnv("EVENT_DOMAIN", "")
	sourceIP := getEnv("EVENT_SOURCE_IP", "")
	filterOutSourceIP := getEnv("EVENT_SOURCE_IP_NOT", "")
	startTime := time.Now()
	endTime := time.Now().Add(-24 * time.Hour)
	readonly := getEnv("EVENT_READONLY", "")
	limit, _ := strconv.Atoi(getEnv("EVENT_LIMIT", "20"))

	//// Example of cached resources
	//cacheTtl := 86400 * time.Second
	//dataCache, err := getCache(ctx, cacheTtl)
	//if err != nil {
	//	return errors.New(err)
	//}

	// cloudtrail query
	lookup := cloudtrail.NewLookupMiddleware().
		WithStartTime(startTime).
		WithLimit(int32(limit)).
		WithEndTime(endTime)

	if resourceId != "" {
		lookup = lookup.WithResourceId(resourceId)
	}

	if resourceType != "" {
		lookup = lookup.WithResourceType(types.ResourceType(resourceType))
	}

	if username != "" {
		lookup = lookup.WithUsername(username)
	}

	if eventName != "" {
		lookup = lookup.WithEventName(eventName)
	}

	if readonly != "" {
		lookup = lookup.WithReadOnly(readonly)
	}

	if errs, ok := lookup.Errors(); !ok {
		return slice.Head(errs).OrEmpty()
	}

	providers, err := v2.DefaultAwsClientProviders()
	if err != nil {
		fin(err)
	}

	clientPool := provider.NewClientPool(ctx, v2.NewClientBuilder(ctx, providers...))
	clients, err := clientPool.GetClients(awsRegions...)
	if err != nil {
		return errors.New(err)
	}

	events := make(chan cloudtrail.Event, 50)
	errChan := make(chan *errors.Error, 50)
	defer close(events)
	defer close(errChan)

	for _, client := range clients {
		ctx2, cancel := context.WithCancel(ctx)

		//// Example of cached events
		//events, err := cloudtrail.
		//	NewCloudTrailRepository(ctx2, client).
		//	ListEventsByLookupCached(dataCache, lookup)

		cloudtrail.
			NewCloudTrailRepository(ctx2, client).
			ListEventsByLookupAsync(lookup, events, errChan)

		service.CancelContextOnError(ctx2, cancel, errChan)

		log.Info().
			Stringer("accountID", client.GetAccountID()).
			Stringer("region", client.GetRegion()).
			Int("events", len(events)).
			Str("id", resourceId).
			Str("type", resourceType).
			Str("user", username).
			Str("domain", domain).
			Str("sourceIP", sourceIP).
			Str("readonly", readonly).
			Str("event", eventName).
			Time("start", startTime).
			Time("end", endTime).
			Int("limit", limit).
			Msg("CloudTrail events")

		filteredOutCount := 0
		for e := range events {
			if readonly != "" && e.GetReadOnly() != readonly {
				log.Trace().Msg("Skipped event due to readonly filter")
				filteredOutCount++
				continue
			}

			if domain != "" && !strings.Contains(e.GetUsername(), domain) {
				log.Trace().Msg("Skipped event due to user email domain filter")
				filteredOutCount++
				continue
			}

			if sourceIP != "" && e.GetSourceIPAddress() != sourceIP {
				log.Trace().Str(sourceIP, e.GetSourceIPAddress()).Msg("Skipped event due to source IP address filter")
				filteredOutCount++
				continue
			}

			if filterOutSourceIP != "" && e.GetSourceIPAddress() == filterOutSourceIP {
				log.Trace().Str("sourceIpAddress", e.GetSourceIPAddress()).Msg("Skipped event due to source IP address filter out")
				filteredOutCount++
				continue
			}

			log.Info().
				Str("id", e.GetId()).
				Str("name", e.GetName()).
				Str("source", e.GetSource()).
				Str("sourceIP", e.GetSourceIPAddress()).
				Str("user", e.GetUsername()).
				Str("readonly", e.GetReadOnly()).
				Time("createAt", e.GetCreatedAt()).
				Msg("cloudtrail event")
		}

		log.Debug().Int("filteredOut", filteredOutCount).Msg("Filtered out events")
	}

	return nil
}

func getCache(ctx context.Context, ttl time.Duration) (*cache.DataCache, error) {
	inFile, err := handlers.NewInFile("/tmp", ttl)
	if err != nil {
		return nil, errors.New(err)
	}

	bigCache, err := bigcache.New(ctx, bigcache.DefaultConfig(ttl))
	if err != nil {
		return nil, errors.New(err)
	}
	inMemory := handlers.NewInMemory(bigCache)

	// data cache
	dataCache := cache.NewDataCache().WithHandlers(inMemory, inFile)

	return dataCache, nil
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}

	return fallback
}

func fin(err error) {
	fmt.Printf("Error: %s\n", err.Error())
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
