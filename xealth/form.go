package xealth

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"github.com/mitchellh/mapstructure"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"net/mail"
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
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Email     string `json:"email"`
}
type GuardianFormData struct {
	Guardian Guardian `json:"guardian"`
	Dexcom   Dexcom   `json:"dexcom"`
}

func (g GuardianFormData) Normalize() PreorderFormData {
	return PreorderFormData{
		Guardian: &g.Guardian,
		Dexcom:   g.Dexcom,
	}
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

func (g *PreorderFormErrors) GetTitle() string {
	return g.Title
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

type Patient struct {
	Email string `json:"email"`
}

type PatientFormData struct {
	Patient Patient `json:"patient"`
	Dexcom  Dexcom  `json:"dexcom"`
}

func (p PatientFormData) Normalize() PreorderFormData {
	return PreorderFormData{
		Patient: &p.Patient,
		Dexcom:  p.Dexcom,
	}
}

type Dexcom struct {
	Connect bool `json:"connect"`
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
	UiSchema   any                `json:"uiSchema,omitempty"`
}

type FormSchemaOverride struct {
	Title      string `json:"title,omitempty"`
	Properties any    `json:"properties,omitempty"`
}

type UiSchema struct {
	UiOrder []string `json:"ui:order,omitempty"`
}

type PreorderFormData struct {
	Id             *primitive.ObjectID `bson:"_id,omitempty"`
	DataTrackingId string              `bson:"dataTrackingId,omitempty"`
	Patient        *Patient            `bson:"patient,omitempty"`
	Guardian       *Guardian           `bson:"guardian,omitempty"`
	Dexcom         Dexcom              `bson:"dexcom"`
}

func isValidEmail(email string) bool {
	_, err := mail.ParseAddress(email)
	return err == nil
}
