package emrserverless

import (
	"encoding/gob"

	"github.com/aws/aws-sdk-go-v2/service/emrserverless/types"
)

// init registers the SDK union members that are not declared in this package but appear in
// interface-typed fields of the types that are (types.JobRun.JobDriver). gob only needs an
// explicit registration for values held in an interface field — concrete types are encoded by
// reflection — so this list is deliberately narrow.
func init() {
	gob.Register(types.JobDriverMemberSparkSubmit{})
	gob.Register(types.JobDriverMemberHive{})
}
