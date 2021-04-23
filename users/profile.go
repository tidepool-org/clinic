package users

type Profile struct {
	FullName *string `json:"fullName"`
	Patient  Patient `json:"patient"`
}

type Patient struct {
	Mrn           *string   `json:"mrn"`
	Birthday      *string   `json:"birthday"`
	TargetDevices *[]string `json:"targetDevices"`
	Email         *string   `json:"email"`
}
