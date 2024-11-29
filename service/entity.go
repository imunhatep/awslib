package service

type EntityInterface interface {
	ResourceInterface
	GetName() string
	GetTags() map[string]string
	GetTagValue(string) string
}
