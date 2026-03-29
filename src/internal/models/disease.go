package models

import "time"

// InheritancePattern corresponds to SQL enum inheritance_pattern

type InheritancePattern string

const (
	InheritanceAD      InheritancePattern = "AD"
	InheritanceAR      InheritancePattern = "AR"
	InheritanceXLD     InheritancePattern = "XLD"
	InheritanceXLR     InheritancePattern = "XLR"
	InheritanceMT      InheritancePattern = "MT"
	InheritanceMULTI   InheritancePattern = "MULTI"
	InheritanceUNKNOWN InheritancePattern = "UNKNOWN"
)

// Disease corresponds to the disease table

type Disease struct {
	DiseaseID          int                `db:"disease_id" json:"disease_id"`
	DiseaseName        string             `db:"disease_name" json:"disease_name"`
	OmimPhenotypeID    *string            `db:"omim_phenotype_id" json:"omim_phenotype_id,omitempty"`
	OrphaCode          *string            `db:"orpha_code" json:"orpha_code,omitempty"`
	InheritancePattern InheritancePattern `db:"inheritance_pattern" json:"inheritance_pattern"`
	Description        *string            `db:"description" json:"description,omitempty"`
	CreatedAt          time.Time          `db:"created_at" json:"created_at"`
	UpdatedAt          time.Time          `db:"updated_at" json:"updated_at"`
}
