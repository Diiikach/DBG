package models

import "time"

// ZygosityType corresponds to SQL enum zygosity_type
// Values: 'heterozygous','homozygous','hemizygous'

type ZygosityType string

const (
	ZygosityHeterozygous ZygosityType = "heterozygous"
	ZygosityHomozygous   ZygosityType = "homozygous"
	ZygosityHemizygous   ZygosityType = "hemizygous"
)

// PatientVariant represents patient_variant table

type PatientVariant struct {
	PatientVariantID int64         `db:"patient_variant_id" json:"patient_variant_id"`
	PatientID        int           `db:"patient_id" json:"patient_id"`
	VariantID        int64         `db:"variant_id" json:"variant_id"`
	SampleID         *int          `db:"sample_id" json:"sample_id,omitempty"`
	Zygosity         *ZygosityType `db:"zygosity" json:"zygosity,omitempty"`
	Quality          *float64      `db:"quality" json:"quality,omitempty"`
	ReadDepth        *int          `db:"read_depth" json:"read_depth,omitempty"`
	AlleleDepthRef   *int          `db:"allele_depth_ref" json:"allele_depth_ref,omitempty"`
	AlleleDepthAlt   *int          `db:"allele_depth_alt" json:"allele_depth_alt,omitempty"`
	GenotypeQuality  *int          `db:"genotype_quality" json:"genotype_quality,omitempty"`
	FilterStatus     *string       `db:"filter_status" json:"filter_status,omitempty"`
	DetectedAt       *time.Time    `db:"detected_at" json:"detected_at,omitempty"`
	CreatedAt        time.Time     `db:"created_at" json:"created_at"`
}
