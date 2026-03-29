package models

import "time"

type Sex string

const (
	SexMale   Sex = "male"
	SexFemale Sex = "female"
	SexOther  Sex = "other"
)

type Patient struct {
	PatientID           int        `db:"patient_id" json:"patient_id"`
	ExternalID          string     `db:"external_id" json:"external_id"`
	FirstName           string     `db:"first_name" json:"first_name"`
	LastName            string     `db:"last_name" json:"last_name"`
	DateOfBirth         time.Time  `db:"date_of_birth" json:"date_of_birth"`
	Sex                 Sex        `db:"sex" json:"sex"`
	PhoneNumber         *string    `db:"phone_number" json:"phone_number,omitempty"`
	Email               *string    `db:"email" json:"email,omitempty"`
	Address             *string    `db:"address" json:"address,omitempty"`
	PhenotypeDescription *string   `db:"phenotype_description" json:"phenotype_description,omitempty"`
	CreatedAt           time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt           time.Time  `db:"updated_at" json:"updated_at"`
}
