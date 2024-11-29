package resources

import (
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/imunhatep/awslib/gateway"
	"github.com/imunhatep/awslib/service/cfg"
	"github.com/imunhatep/gocollection/slice"
	"github.com/rs/zerolog/log"
)

type HandlerFunc func(c *ResourceReader) error

type MiddlewareInterface interface {
	HandleResourceReader(next HandlerFunc) HandlerFunc
}

type ProviderInterface interface {
	Run() *ResourceReader
}

type ResourceObserver struct {
	gatewayPool *gateway.RepoGatewayPool
	providers   map[types.ResourceType]ProviderInterface

	handler    HandlerFunc
	middleware []MiddlewareInterface
}

// NewResourceObserver creates a new resource handler
func NewResourceObserver(gatewayPool *gateway.RepoGatewayPool, handler HandlerFunc) *ResourceObserver {
	return &ResourceObserver{
		gatewayPool: gatewayPool,
		providers:   map[types.ResourceType]ProviderInterface{},
		handler:     handler,
		middleware:  []MiddlewareInterface{},
	}
}

// Use adds middleware to the resource handler
func (r *ResourceObserver) Use(middlewares ...MiddlewareInterface) {
	log.Debug().
		Strs("middleware", slice.Map(middlewares, func(v MiddlewareInterface) string { return fmt.Sprintf("%T", v) })).
		Msg("[ResourceObserver.Use] adding middleware")

	r.middleware = append(r.middleware, middlewares...)
}

// Handler returns the handler function
func (r *ResourceObserver) Handler() HandlerFunc {
	return r.handler
}

// Serve runs the resource handler
func (r *ResourceObserver) Serve(resourceTypes []types.ResourceType) error {
	var h HandlerFunc

	h = r.Handler()
	h = applyMiddleware(h, r.middleware...)

	// runs aws api requests synchronously per resource type and asynchronously per region
	for _, resourceType := range resourceTypes {
		log.Trace().
			Str("type", cfg.ResourceTypeToString(resourceType)).
			Msg("[ResourceObserver.Serve] processing resource")

		// Run resource type observer
		resourceReader := r.getProvider(resourceType).Run()

		// Execute chain
		if err := h(resourceReader); err != nil {
			log.Error().Err(err).Msg("Error processing resources")
			return err
		}
	}

	return nil
}

func (r *ResourceObserver) getProvider(resourceType types.ResourceType) ProviderInterface {
	if _, ok := r.providers[resourceType]; !ok {
		r.providers[resourceType] = NewProvider(resourceType, r.gatewayPool.List(resourceType)...)
	}

	return r.providers[resourceType]
}

// applyMiddleware applies middleware to the handler
func applyMiddleware(h HandlerFunc, middleware ...MiddlewareInterface) HandlerFunc {
	for i := len(middleware) - 1; i >= 0; i-- {
		h = middleware[i].HandleResourceReader(h)
	}
	return h
}
