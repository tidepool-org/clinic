package test

import (
	"github.com/tidepool-org/clinic/clinics"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func Tags() []clinics.PatientTag {
	firstId, _ := primitive.ObjectIDFromHex("000000000000000000000000")
	secondId, _ := primitive.ObjectIDFromHex("000000000000000000000001")

	return []clinics.PatientTag{
		{
			Id:   &firstId,
			Name: "Tidepool Loop",
		},
		{
			Id:   &secondId,
			Name: "DIY Loop",
		},
	}
}
