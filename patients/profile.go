package patients

type Profile struct {
	FullName *string        `json:"fullName"`
	Patient  PatientProfile `json:"patient"`
}

type PatientProfile struct {
	Mrn           *string   `json:"mrn"`
	Birthday      *string   `json:"birthday"`
	TargetDevices *[]string `json:"targetDevices"`
	Email         *string   `json:"email"`
	FullName      *string   `json:"fullName"`
}
