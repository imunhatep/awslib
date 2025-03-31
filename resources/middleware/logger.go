package middleware

import (
	"encoding/json"
	"github.com/imunhatep/awslib/resources"
	"github.com/imunhatep/awslib/service/cfg"
	"github.com/rs/zerolog/log"
)

type LoggerMiddleware struct{}

func NewLoggerMiddleware() *LoggerMiddleware {
	return &LoggerMiddleware{}
}

// HandleResourceReader is a middleware that processes resources from the resource reader
func (m *LoggerMiddleware) HandleResourceReader(next resources.HandlerFunc) resources.HandlerFunc {
	return func(reader resources.ResourceReaderInterface) error {
		resourceType := reader.ResourceType()
		resourceList := reader.Read()

		for _, resource := range resourceList {
			tags, _ := json.Marshal(resource.GetTags())
			log.Trace().
				Str("type", cfg.ResourceTypeToString(resourceType)).
				Str("arn", resource.GetArn()).
				Str("tags", string(tags)).
				Msg("[LoggerMiddleware] resource found")
		}

		return next(reader)
	}
}
