package xealth_client

// The following models were manually added, because oapi-codegen@v2.0.0 doesn't generate the required types
// when an enum is defined inline. This code can be removed when https://github.com/deepmap/oapi-codegen/issues/1305
// is resolved.

type PreorderFormRequest0EventContext string

const (
	PreorderFormRequest0EventContextInitial PreorderFormRequest0EventContext = "initial"
)

type PreorderFormRequest1EventContext string

const (
	PreorderFormRequest1EventContextSubsequent PreorderFormRequest0EventContext = "subsequent"
)

type PreorderFormRequest0EventType string

const (
	PreorderFormRequest0EventTypePreorder PreorderFormRequest0EventType = "preorder"
)

type PreorderFormRequest1EventType string

const (
	PreorderFormRequest1EventTypePreorder PreorderFormRequest1EventType = "preorder"
)

type ProviderMessageRequestMessage0Priority string
type ProviderMessageRequestMessage0Type string
type ProviderMessageRequestMessage1Type string
