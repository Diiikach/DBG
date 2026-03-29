package models

import "time"

// ImpactType corresponds to SQL enum impact_type
// Values: 'HIGH','MODERATE','LOW','MODIFIER'

type ImpactType string

const (
	ImpactHigh     ImpactType = "HIGH"
	ImpactModerate ImpactType = "MODERATE"
	ImpactLow      ImpactType = "LOW"
	ImpactModifier ImpactType = "MODIFIER"
)

// SiftPredictionType corresponds to SQL enum sift_prediction_type
// Values: 'tolerated','deleterious'

type SiftPredictionType string

const (
	SiftPredictionTolerated   SiftPredictionType = "tolerated"
	SiftPredictionDeleterious SiftPredictionType = "deleterious"
)

// PolyphenPredictionType corresponds to SQL enum polyphen_prediction_type
// Values: 'benign','possibly_damaging','probably_damaging'

type PolyphenPredictionType string

const (
	PolyphenPredictionBenign           PolyphenPredictionType = "benign"
	PolyphenPredictionPossiblyDamaging PolyphenPredictionType = "possibly_damaging"
	PolyphenPredictionProbablyDamaging PolyphenPredictionType = "probably_damaging"
)

// ClinvarSignificanceType corresponds to SQL enum clinvar_significance_type

type ClinvarSignificanceType string

const (
	ClinvarBenign           ClinvarSignificanceType = "benign"
	ClinvarLikelyBenign     ClinvarSignificanceType = "likely_benign"
	ClinvarUncertain        ClinvarSignificanceType = "uncertain_significance"
	ClinvarLikelyPathogenic ClinvarSignificanceType = "likely_pathogenic"
	ClinvarPathogenic       ClinvarSignificanceType = "pathogenic"
	ClinvarConflicting      ClinvarSignificanceType = "conflicting"
	ClinvarNotProvided      ClinvarSignificanceType = "not_provided"
)

// VariantAnnotation represents the variant_annotation table

type VariantAnnotation struct {
	AnnotationID        int64                    `db:"annotation_id" json:"annotation_id"`
	VariantID           int                      `db:"variant_id" json:"variant_id"`
	GeneID              *int                     `db:"gene_id" json:"gene_id,omitempty"`
	Consequence         *string                  `db:"consequence" json:"consequence,omitempty"`
	Impact              *ImpactType              `db:"impact" json:"impact,omitempty"`
	TranscriptID        *string                  `db:"transcript_id" json:"transcript_id,omitempty"`
	Hgvsc               *string                  `db:"hgvsc" json:"hgvsc,omitempty"`
	Hgvsp               *string                  `db:"hgvsp" json:"hgvsp,omitempty"`
	ProteinPosition     *int                     `db:"protein_position" json:"protein_position,omitempty"`
	AminoAcids          *string                  `db:"amino_acids" json:"amino_acids,omitempty"`
	Codons              *string                  `db:"codons" json:"codons,omitempty"`
	GnomadAF            *float64                 `db:"gnomad_af" json:"gnomad_af,omitempty"`
	GnomadAFPopmax      *float64                 `db:"gnomad_af_popmax" json:"gnomad_af_popmax,omitempty"`
	SiftScore           *float64                 `db:"sift_score" json:"sift_score,omitempty"`
	SiftPrediction      *SiftPredictionType      `db:"sift_prediction" json:"sift_prediction,omitempty"`
	PolyphenScore       *float64                 `db:"polyphen_score" json:"polyphen_score,omitempty"`
	PolyphenPrediction  *PolyphenPredictionType  `db:"polyphen_prediction" json:"polyphen_prediction,omitempty"`
	CaddScore           *float64                 `db:"cadd_score" json:"cadd_score,omitempty"`
	RevelScore          *float64                 `db:"revel_score" json:"revel_score,omitempty"`
	ClinvarSignificance *ClinvarSignificanceType `db:"clinvar_significance" json:"clinvar_significance,omitempty"`
	ClinvarID           *string                  `db:"clinvar_id" json:"clinvar_id,omitempty"`
	CreatedAt           time.Time                `db:"created_at" json:"created_at"`
}
