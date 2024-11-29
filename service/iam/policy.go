package iam

import (
	"encoding/gob"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	cfg "github.com/aws/aws-sdk-go-v2/service/configservice/types"
	"github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/imunhatep/awslib/service"
)

func init() {
	gob.Register(Policy{})
	gob.Register(PolicyVersion{})
	gob.Register(PolicyDocument{})
	gob.Register(Statement{})
}

type PolicyList struct {
	Items []Policy
}

type Policy struct {
	service.AbstractResource
	types.Policy
}

func NewPolicy(client AwsClient, policy types.Policy) Policy {
	usrArn, _ := arn.Parse(aws.ToString(policy.Arn))

	return Policy{
		AbstractResource: service.AbstractResource{
			AccountID: client.GetAccountID(),
			Region:    client.GetRegion(),
			ID:        aws.ToString(policy.PolicyId),
			ARN:       &usrArn,
			CreatedAt: aws.ToTime(policy.CreateDate),
			Type:      cfg.ResourceTypePolicy,
		},
		Policy: policy,
	}
}

func (e Policy) GetName() string {
	return aws.ToString(e.PolicyName)
}

func (e Policy) GetTags() map[string]string {
	tags := make(map[string]string)

	for _, tag := range e.Tags {
		tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}

	return tags
}

func (e Policy) GetTagValue(tag string) string {
	val, ok := e.GetTags()[tag]
	if !ok {
		return ""
	}

	return val
}

type PolicyDocument struct {
	Version   string      `json:"Version"`
	Statement []Statement `json:"Statement"`
}

type Statement struct {
	Effect   string      `json:"Effect"`
	Action   interface{} `json:"Action"`
	Resource interface{} `json:"Resource"`
}

type PolicyVersion struct {
	types.PolicyVersion
	document PolicyDocument
}

func NewPolicyVersion(policyVersion types.PolicyVersion, document PolicyDocument) PolicyVersion {
	return PolicyVersion{
		PolicyVersion: policyVersion,
		document:      document,
	}
}

func (p PolicyVersion) GetDocument() PolicyDocument {
	return p.document
}
