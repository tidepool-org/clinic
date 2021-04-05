package authz

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"github.com/fatih/structs"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/rego"
	"github.com/tidepool-org/clinic/clinicians"
	internalErrs "github.com/tidepool-org/clinic/errors"

	"net/http"
	"strings"
)

const (
	subjectIdHeaderName   = "x-auth-subject-id"
	serverAccessHeaderKey = "x-auth-server-access"
	clinicIdPathParameter = "clinicId"
)

var (
	//go:embed policy.rego
	authzPolicy string

	ErrUnauthorized = errors.New("the subject is not authorized for the requested action")
)

type RequestAuthorizer interface {
	Authorize(context.Context, *openapi3filter.AuthenticationInput) error
}

func NewRequestAuthorizer(clinicians clinicians.Service) (RequestAuthorizer, error) {
	compiler, err := ast.CompileModules(map[string]string{
		"policy.rego": authzPolicy,
	})
	if err != nil {
		return nil, err
	}

	return &embeddedOpaAuthorizer{
		clinicians: clinicians,
		policy:     compiler,
	}, nil
}

type embeddedOpaAuthorizer struct {
	clinicians clinicians.Service
	policy     *ast.Compiler
}

func (e *embeddedOpaAuthorizer) Authorize(ctx context.Context, input *openapi3filter.AuthenticationInput) error {
	clinician, err := e.getClinicianRecord(ctx, input)
	if err != nil {
		return err
	}

	in := map[string]interface{}{
		"headers": e.getHeaders(input),
		"path": strings.Split(input.RequestValidationInput.Route.Path, "/"),
		"method": strings.ToUpper(input.RequestValidationInput.Request.Method),
		"clinician": structs.Map(clinician),
	}

	r := rego.New(
		rego.Package("http.authz.clinic"),
		rego.Query("allow"),
		rego.Compiler(e.policy),
		rego.Input(in),
	)

	results, err := r.Eval(ctx)
	if err != nil {
		return fmt.Errorf("unable to evaluate authorization policy: %w", err)
	}

	if len(results) == 0 || len(results[0].Expressions) == 0 {
		return fmt.Errorf("evaluating authorization policy return no results")
	}

	val, ok := results[0].Expressions[0].Value.(bool)
	if !ok {
		return fmt.Errorf("unexpected authorization result: %v", results[0].Expressions[0].Value)
	}

	if !val {
		return ErrUnauthorized
	}

	return nil
}

func (e *embeddedOpaAuthorizer) getHeaders(input *openapi3filter.AuthenticationInput) map[string]string {
	return map[string]string{
		subjectIdHeaderName:   input.RequestValidationInput.Request.Header.Get(subjectIdHeaderName),
		serverAccessHeaderKey: input.RequestValidationInput.Request.Header.Get(serverAccessHeaderKey),
	}
}

// Get the clinician record for the currently authenticated user
func (e *embeddedOpaAuthorizer) getClinicianRecord(ctx context.Context, input *openapi3filter.AuthenticationInput) (*clinicians.Clinician, error) {
	clinicId := input.RequestValidationInput.PathParams[clinicIdPathParameter]
	if clinicId == "" {
		return nil, nil
	}
	currentUserId := GetAuthUserId(input.RequestValidationInput.Request)
	if currentUserId == nil {
		return nil, nil
	}
	clinician, err := e.clinicians.Get(ctx, clinicId, *currentUserId)
	if !errors.Is(err, internalErrs.NotFound) {
		return nil, err
	}

	return clinician, nil
}

func GetAuthUserId(r *http.Request) *string {
	headers := r.Header
	if headers.Get(serverAccessHeaderKey) == "true" {
		return nil
	}
	subjectId := headers.Get(subjectIdHeaderName)
	if subjectId == "" {
		return nil
	}

	return &subjectId
}
