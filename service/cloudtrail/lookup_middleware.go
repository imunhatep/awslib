package cloudtrail

import (
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/go-errors/errors"
	"github.com/imunhatep/awslib/service"
	"github.com/imunhatep/gocollection/helper"
	"github.com/imunhatep/gocollection/slice"
	"github.com/rs/zerolog/log"
	"strings"
	"time"
)

type LookupHandler func(*cloudtrail.LookupEventsInput) (*cloudtrail.LookupEventsInput, error)

type LookupMiddleware struct {
	lookup *cloudtrail.LookupEventsInput
	errs   []error
}

func NewLookupMiddleware() *LookupMiddleware {
	return &LookupMiddleware{
		lookup: &cloudtrail.LookupEventsInput{},
		errs:   []error{},
	}
}

func (l *LookupMiddleware) Get() *cloudtrail.LookupEventsInput {
	return l.lookup
}

func (l *LookupMiddleware) Hash() string {
	optStr := []string{}
	for _, q := range l.lookup.LookupAttributes {
		optStr = append(optStr, fmt.Sprintf("%s:%s", string(q.AttributeKey), aws.ToString(q.AttributeValue)))
	}

	optStr = append(optStr, fmt.Sprintf("EventCategory:%s", l.lookup.EventCategory))
	optStr = append(optStr, fmt.Sprintf("StartTime:%s", l.lookup.StartTime))
	optStr = append(optStr, fmt.Sprintf("EndTime:%s", l.lookup.EndTime))
	optStr = append(optStr, fmt.Sprintf("MaxResults:%d", aws.ToInt32(l.lookup.MaxResults)))

	log.Debug().Strs("hash", optStr).Msg("[LookupMiddleware.Hash] lookup string")

	hasher := sha1.New()
	hasher.Write([]byte(strings.Join(slice.Sort(optStr, helper.StrSort), ".")))

	return base64.URLEncoding.EncodeToString(hasher.Sum(nil))
}

func (l *LookupMiddleware) Errors() ([]error, bool) {
	return l.errs, len(l.errs) == 0
}

func (l *LookupMiddleware) WithHandler(f LookupHandler) *LookupMiddleware {
	var err error
	if l.lookup, err = f(l.lookup); err != nil {
		l.errs = append(l.errs, err)
	}

	return l
}

func (l *LookupMiddleware) WithStartTime(start time.Time) *LookupMiddleware {
	return l.WithHandler(LookupStartTimeHandler(start))
}

func (l *LookupMiddleware) WithEndTime(end time.Time) *LookupMiddleware {
	return l.WithHandler(LookupEndTimeHandler(end))
}

func (l *LookupMiddleware) WithLimit(limit int32) *LookupMiddleware {
	return l.WithHandler(LookupLimitHandler(limit))
}

func (l *LookupMiddleware) WithEventName(value string) *LookupMiddleware {
	return l.WithHandler(LookupEventByAttribute(types.LookupAttributeKeyEventName, value))
}

func (l *LookupMiddleware) WithResourceType(value cfg.ResourceType) *LookupMiddleware {
	return l.WithHandler(LookupEventByAttribute(types.LookupAttributeKeyResourceType, string(value)))
}

func (l *LookupMiddleware) WithResource(value service.EntityInterface) *LookupMiddleware {
	return l.WithHandler(LookupResourceHandler(value))
}

func (l *LookupMiddleware) WithResourceId(value string) *LookupMiddleware {
	return l.WithHandler(LookupEventByAttribute(types.LookupAttributeKeyResourceName, value))
}

func (l *LookupMiddleware) WithReadOnly(value string) *LookupMiddleware {
	return l.WithHandler(LookupEventByAttribute(types.LookupAttributeKeyReadOnly, value))
}

func (l *LookupMiddleware) WithUsername(value string) *LookupMiddleware {
	return l.WithHandler(LookupEventByAttribute(types.LookupAttributeKeyUsername, value))
}

func LookupEventByAttribute(key types.LookupAttributeKey, value string) LookupHandler {
	return func(q *cloudtrail.LookupEventsInput) (*cloudtrail.LookupEventsInput, error) {
		log.Trace().
			Str("key", string(key)).
			Str("value", value).
			Msg("[CloudTrail.LookupEventByAttribute]")

		eventName := types.LookupAttribute{
			AttributeKey:   key,
			AttributeValue: &value,
		}

		if len(q.LookupAttributes) < 1 {
			q.LookupAttributes = append(q.LookupAttributes, eventName)
		} else {
			log.Warn().
				Str("key", string(key)).
				Str("value", value).
				Msg("[CloudTrail.LookupEventByAttribute] skipped. Only 1 lookup attribute is supported, refer to AWS CloudTrail SDK docs")

			return q, errors.New("only 1 lookup attribute is supported, refer to AWS CloudTrail SDK docs")
		}

		return q, nil
	}
}

func LookupStartTimeHandler(t time.Time) LookupHandler {
	return func(q *cloudtrail.LookupEventsInput) (*cloudtrail.LookupEventsInput, error) {
		log.Trace().Str("key", "StartTime").Time("time", t).Msg("lookup: query")

		q.StartTime = &t

		return q, nil
	}
}

func LookupEndTimeHandler(t time.Time) LookupHandler {
	return func(q *cloudtrail.LookupEventsInput) (*cloudtrail.LookupEventsInput, error) {
		log.Trace().Str("key", "EndTime").Time("time", t).Msg("lookup: query")

		q.EndTime = &t

		return q, nil
	}
}

func LookupLimitHandler(limit int32) LookupHandler {
	return func(q *cloudtrail.LookupEventsInput) (*cloudtrail.LookupEventsInput, error) {
		log.Trace().Str("key", "MaxResults").Int32("limit", limit).Msg("lookup: query")

		q.MaxResults = &limit

		return q, nil
	}
}

func LookupResourceHandler(e service.EntityInterface) LookupHandler {
	return func(q *cloudtrail.LookupEventsInput) (*cloudtrail.LookupEventsInput, error) {
		log.Trace().Str("key", "ResourceName").Str("name", e.GetIdOrArn()).Msg("lookup: query")

		q, _ = LookupEventByAttribute(types.LookupAttributeKeyResourceName, e.GetIdOrArn())(q)

		return q, nil
	}
}

func DebugQuery(msg string, query *cloudtrail.LookupEventsInput) {
	info := log.Debug()

	if query.StartTime != nil {
		info.Time("start", *query.StartTime)
	}

	if query.EndTime != nil {
		info.Time("end", *query.EndTime)
	}

	if query.MaxResults != nil {
		info.Int32("limit", *query.MaxResults)
	}

	if !slice.IsEmpty(query.LookupAttributes) {
		attr, _ := slice.Head(query.LookupAttributes).Get()
		info.Str("attribute", fmt.Sprintf("%v", attr))
	}

	for _, attr := range query.LookupAttributes {
		info.Str(string(attr.AttributeKey), *attr.AttributeValue)
	}

	info.Msg(msg)
}
