package models

import "time"


type ACMGClassification string

const (
	ACMGPathogenic            ACMGClassification = "pathogenic"
	ACMGLikelyPathogenic      ACMGClassification = "likely_pathogenic"
	ACMGUncertainSignificance ACMGClassification = "uncertain_significance"
	ACMGLikelyBenign          ACMGClassification = "likely_benign"
	ACMGBenign                ACMGClassification = "benign"
)

// VariantInterpretation represents the variant_interpretation table

type VariantInterpretation struct {
	InterpretationID   int64              `db:"interpretation_id" json:"interpretation_id"`
	PatientVariantID   int64              `db:"patient_variant_id" json:"patient_variant_id"`
	UserID             int                `db:"user_id" json:"user_id"`
	ACMGClassification ACMGClassification `db:"acmg_classification" json:"acmg_classification"`
	ACMGCriteria       *string            `db:"acmg_criteria" json:"acmg_criteria,omitempty"`
	InterpretationText *string            `db:"interpretation_text" json:"interpretation_text,omitempty"`
	DiseaseID          *int               `db:"disease_id" json:"disease_id,omitempty"`
	CreatedAt          time.Time          `db:"created_at" json:"created_at"`
	UpdatedAt          time.Time          `db:"updated_at" json:"updated_at"`
}
