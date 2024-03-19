package config

import (
	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	ClinicDemoPatientUserId string `envconfig:"CLINIC_DEMO_PATIENT_USER_ID"`
}

func NewConfig() (*Config, error) {
	c := &Config{}
	err := envconfig.Process("", c)
	return c, err
}
