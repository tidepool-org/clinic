package xealth

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"github.com/mitchellh/mapstructure"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"strings"
)

const (
	FormTitlePatientNameTemplate = "Add %s to Tidepool"
)

//go:embed forms/patient_enrollment_form.json
var patientEnrollmentForm []byte

//go:embed forms/guardian_enrollment_form.json
var guardianEnrollmentForm []byte

//go:embed forms/error_form.json
var errorForm []byte

type DataValidator[D FormData] interface {
	Validate(D) (FormErrors, error)
}

type FormData interface {
	Normalize() PreorderFormData
}

type FormErrors interface {
	GetTitle() string
	HasErrors() bool
	GetErrorProperties() map[string]PreorderFormErrorParagraph
	GetUiOrder() []string
}

type Guardian struct {
	FirstName     string `json:"firstName,omitempty"`
	LastName      string `json:"lastName,omitempty"`
	Email         string `json:"email,omitempty"`
	ConnectAbbott bool   `json:"connectAbbott,omitempty"`
	ConnectDexcom bool   `json:"connectDexcom,omitempty"`
}
type GuardianFormData struct {
	Guardian Guardian `json:"guardian"`
	Tags     *Tags    `json:"tags,omitempty"`
}

func (g GuardianFormData) Normalize() PreorderFormData {
	if strings.TrimSpace(g.Guardian.Email) == "" {
		g.Guardian.ConnectDexcom = false
	}

	return PreorderFormData{
		Guardian: &g.Guardian,
		Tags:     g.Tags,
	}
}

type Patient struct {
	Email         string `json:"email,omitempty"`
	ConnectAbbott bool   `json:"connectAbbott,omitempty"`
	ConnectDexcom bool   `json:"connectDexcom,omitempty"`
}

type PatientFormData struct {
	Patient Patient `json:"patient"`
	Tags    *Tags   `json:"tags,omitempty"`
}

func (p PatientFormData) Normalize() PreorderFormData {
	if strings.TrimSpace(p.Patient.Email) == "" {
		p.Patient.ConnectDexcom = false
	}

	return PreorderFormData{
		Patient: &p.Patient,
		Tags:    p.Tags,
	}
}

type Tags struct {
	Ids []string `json:"ids,omitempty"`
}

type TagDefinitions struct {
	Enums     []string `json:"enums"`
	EnumNames []string `json:"enumNames"`
}

type PreorderFormErrors struct {
	Title      string
	uiOrder    []string
	paragraphs map[string]PreorderFormErrorParagraph
}

func (p *PreorderFormErrors) AddErrorParagraph(errorParagraph string) {
	if p.paragraphs == nil {
		p.paragraphs = make(map[string]PreorderFormErrorParagraph)
	}

	key := fmt.Sprintf("error_%d", len(p.uiOrder))
	p.uiOrder = append(p.uiOrder, key)
	p.paragraphs[key] = PreorderFormErrorParagraph(errorParagraph)
}

func (p *PreorderFormErrors) GetTitle() string {
	return p.Title
}

func (p *PreorderFormErrors) GetUiOrder() []string {
	if !p.HasErrors() {
		return make([]string, 0)
	}

	return p.uiOrder
}

func (p *PreorderFormErrors) GetErrorProperties() map[string]PreorderFormErrorParagraph {
	if !p.HasErrors() {
		return make(map[string]PreorderFormErrorParagraph)
	}

	return p.paragraphs
}

func (p *PreorderFormErrors) HasErrors() bool {
	return len(p.uiOrder) > 0
}

type PreorderFormErrorParagraph string

func (p PreorderFormErrorParagraph) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type        string `json:"type"`
		Title       string `json:"title"`
		Description string `json:"description"`
	}{
		Type:        "null",    // Use the 'null' widget
		Title:       " ",       // Do not render a title
		Description: string(p), // Render the error text as description
	})
}

func DecodeFormData[T any](formData *map[string]interface{}) (data T, err error) {
	if formData == nil || len(*formData) == 0 {
		return
	}

	err = mapstructure.Decode(formData, &data)
	return
}

func EncodeFormData[A any](data A) (*map[string]interface{}, error) {
	j, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	formData := make(map[string]interface{})
	err = json.Unmarshal(j, &formData)
	return &formData, err
}

type FormOverrides struct {
	FormSchema FormSchemaOverride `json:"formSchema"`
	UiSchema   UiSchema           `json:"uiSchema,omitempty"`
}

type FormSchemaOverride struct {
	Title       string                 `json:"title,omitempty"`
	Definitions Definitions            `json:"definitions"`
	Properties  map[string]interface{} `json:"properties,omitempty"`
}

type Definitions struct {
	Tags TagsDefinitions `json:"tags"`
}

type TagsDefinitions struct {
	Enum      []string `json:"enum,omitempty"`
	EnumNames []string `json:"enumNames,omitempty"`
}

type UiSchema struct {
	Tags    TagsUiSchema `json:"tags"`
	UiOrder []string     `json:"ui:order,omitempty"`
}

type TagsUiSchema struct {
	UiWidget string `json:"ui:widget,omitempty"`
}

type PreorderFormData struct {
	Id             *primitive.ObjectID `bson:"_id,omitempty"`
	DataTrackingId string              `bson:"dataTrackingId,omitempty"`
	Patient        *Patient            `bson:"patient,omitempty"`
	Guardian       *Guardian           `bson:"guardian,omitempty"`
	Tags           *Tags               `bson:"tags,omitempty"`
}
