package models

import "time"

// VariantType corresponds to SQL enum variant_type
// Values: 'SNV','INS','DEL','INDEL','MNV','CNV','SV','OTHER'
type VariantType string

const (
	VariantTypeSNV   VariantType = "SNV"
	VariantTypeINS   VariantType = "INS"
	VariantTypeDEL   VariantType = "DEL"
	VariantTypeINDEL VariantType = "INDEL"
	VariantTypeMNV   VariantType = "MNV"
	VariantTypeCNV   VariantType = "CNV"
	VariantTypeSV    VariantType = "SV"
	VariantTypeOther VariantType = "OTHER"
)

// GenomeBuild corresponds to gen_build enum
// Values: 'GRCh37','GRCh38'

type GenomeBuild string

const (
	GenomeBuildGRCh37 GenomeBuild = "GRCh37"
	GenomeBuildGRCh38 GenomeBuild = "GRCh38"
)

// Variant represents the variant table

type Variant struct {
	VariantID       int         `db:"variant_id" json:"variant_id"`
	Chromosome      string      `db:"chromosome" json:"chromosome"`
	Position        int64       `db:"position" json:"position"`
	ReferenceAllele string      `db:"reference_allele" json:"reference_allele"`
	AlternateAllele string      `db:"alternate_allele" json:"alternate_allele"`
	RSID            *string     `db:"rs_id" json:"rs_id,omitempty"`
	GenomeBuild     GenomeBuild `db:"genome_build" json:"genome_build"`
	VariantType     VariantType `db:"variant_type" json:"variant_type"`
	CreatedAt       *time.Time  `db:"created_at" json:"created_at,omitempty"`
}
