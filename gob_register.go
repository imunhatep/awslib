package awslib
import (
	"encoding/gob"
	typesEMR "github.com/aws/aws-sdk-go-v2/service/emrserverless/types"
)
var awsServicesGobRegistered = false
//go:generate go run cmd/generate-gob/main.go
func init() {
	GobRegisterAwsServices()
}
func GobRegisterAwsServices() {
	if awsServicesGobRegistered {
		return
	}
	// Register generated services
	registerGeneratedServices()
	// External SDK types that are not part of our service packages
	// but are used within our structs (e.g. interfaces)
	gob.Register(typesEMR.JobDriverMemberSparkSubmit{})
	gob.Register(typesEMR.JobDriverMemberHive{})
	awsServicesGobRegistered = true
}
// GobRegisterAwsServicesAll is deprecated and now just an alias for GobRegisterAwsServices.
// The registration is handled automatically via generated code in gob_register_gen.go.
func GobRegisterAwsServicesAll() {
	GobRegisterAwsServices()
}
