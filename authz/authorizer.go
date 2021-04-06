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
	"go.uber.org/zap"

	"net/http"
	"strings"
)

const (
	authHeaderPrefix = "x-auth-"
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
	EvaluatePolicy(context.Context, map[string]interface{}) error
}

func NewRequestAuthorizer(clinicians clinicians.Service, logger *zap.SugaredLogger) (RequestAuthorizer, error) {
	compiler, err := ast.CompileModules(map[string]string{
		"policy.rego": authzPolicy,
	})
	if err != nil {
		return nil, err
	}

	return &embeddedOpaAuthorizer{
		clinicians: clinicians,
		logger:     logger,
		policy:     compiler,
	}, nil
}

type embeddedOpaAuthorizer struct {
	clinicians clinicians.Service
	logger     *zap.SugaredLogger
	policy     *ast.Compiler
}

func (e *embeddedOpaAuthorizer) Authorize(ctx context.Context, input *openapi3filter.AuthenticationInput) error {
	clinician, err := e.getClinicianRecord(ctx, input)
	if err != nil {
		return err
	}

	in := map[string]interface{}{
		"headers": e.getAuthHeaders(input),
		"path":    e.getSplitPath(input),
		"method":  strings.ToUpper(input.RequestValidationInput.Request.Method),
	}

	if clinician != nil {
		clinicianStruct := structs.New(*clinician)
		clinicianStruct.TagName = "bson"
		in["clinician"] = clinicianStruct.Map()
	}

	return e.EvaluatePolicy(ctx, in)
}

func (e *embeddedOpaAuthorizer) EvaluatePolicy(ctx context.Context, input map[string]interface{}) error {
	r := rego.New(
		rego.Package("http.authz.clinic"),
		rego.Query("allow"),
		rego.Compiler(e.policy),
		rego.Input(input),
	)

	results, err := r.Eval(ctx)
	if err != nil {
		return fmt.Errorf("unable to evaluate authorization policy: %w", err)
	}

	if len(results) == 0 || len(results[0].Expressions) == 0 {
		return fmt.Errorf("evaluating authorization policy returned no results")
	}

	val, ok := results[0].Expressions[0].Value.(bool)
	if !ok {
		return fmt.Errorf("unexpected authorization result: %v", results[0].Expressions[0].Value)
	}

	e.logger.Debugw("authorization policy eval", zap.Any("input", input), zap.Bool("allow", val))

	if !val {
		return ErrUnauthorized
	}

	return nil
}

func (e *embeddedOpaAuthorizer) getAuthHeaders(input *openapi3filter.AuthenticationInput) map[string]string {
	headers := make(map[string]string, 0)
	for k, v := range input.RequestValidationInput.Request.Header {
		if key := strings.ToLower(k); strings.HasPrefix(key, authHeaderPrefix) {
			headers[key] = strings.Join(v, ",")
		}
	}
	return headers
}

func (e *embeddedOpaAuthorizer) getSplitPath(input *openapi3filter.AuthenticationInput) []string {
	path := strings.Split(input.RequestValidationInput.Request.URL.Path, "/")
	if len(path) > 0 && path[0] == "" {
		path = path[1:]
	}
	return path
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
	if err != nil && !errors.Is(err, internalErrs.NotFound) {
		return nil, err
	}

	return clinician, nil
}

func GetAuthUserId(r *http.Request) *string {
	if r.Header.Get(serverAccessHeaderKey) == "true" {
		return nil
	}
	subjectId := r.Header.Get(subjectIdHeaderName)
	if subjectId == "" {
		return nil
	}

	return &subjectId
}
