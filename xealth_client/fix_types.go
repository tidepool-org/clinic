package xealth_client

// The following models were manually added, because oapi-codegen@v2.3.0 doesn't generate the required types
// when an enum is defined inline. This code can be removed when https://github.com/deepmap/oapi-codegen/issues/1305
// is resolved.

const (
	PreorderFormRequest0EventContextInitial PreorderFormRequest0EventContext = "initial"
)

const (
	PreorderFormRequest1EventContextSubsequent PreorderFormRequest0EventContext = "subsequent"
)

type N200PatientIdentityIdsOrigin string
type N200PatientIdentityHistoricalIdsOrigin string
