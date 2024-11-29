package middleware

import (
	"fmt"
	"github.com/imunhatep/awslib/resources"
	"github.com/imunhatep/awslib/service/cfg"
	"github.com/rs/zerolog/log"
	"time"
)

func SummaryHandler() resources.HandlerFunc {
	return func(reader *resources.ResourceReader) error {
		resourceList := reader.Read()
		resourceType := reader.ResourceType()

		log.Debug().
			Str("resource", cfg.ResourceTypeToString(resourceType)).
			Msgf("[SummaryHandler] resources: %d", len(resourceList))

		return nil
	}
}

func LoggerHandler() resources.HandlerFunc {
	return func(reader *resources.ResourceReader) error {
		resourceList := reader.Read()
		resourceType := reader.ResourceType()

		for _, resource := range resourceList {
			log.Info().
				Str("resource", cfg.ResourceTypeToString(resourceType)).
				Msgf("[LoggerHandler] resources: %s", resource.GetArn())
		}

		return nil
	}
}

func WaitHandler(final chan struct{}) resources.HandlerFunc {
	return func(reader *resources.ResourceReader) error {
		log.Debug().Str("len/cap", fmt.Sprintf("%d/%d", len(final), cap(final))).Msgf("[WaitMiddleware] to final <- struct{}{}")
		time.Sleep(3 * time.Second)
		final <- struct{}{}
		return nil
	}
}

func NullHandler() resources.HandlerFunc {
	return func(reader *resources.ResourceReader) error {
		log.Trace().Msgf("[NullHandler] invoked")
		return nil
	}
}
