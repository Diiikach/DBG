package models

import "time"

// AssociationType corresponds to SQL enum in gene_disease

type AssociationType string

const (
	AssociationCausative      AssociationType = "causative"
	AssociationRiskFactor     AssociationType = "risk_factor"
	AssociationModifier       AssociationType = "modifier"
	AssociationSusceptibility AssociationType = "susceptibility"
	AssociationProtective     AssociationType = "protective"
)

// EvidenceLevel corresponds to SQL enum in gene_disease

type EvidenceLevel string

const (
	EvidenceStrong   EvidenceLevel = "strong"
	EvidenceModerate EvidenceLevel = "moderate"
	EvidenceLimited  EvidenceLevel = "limited"
	EvidenceDisputed EvidenceLevel = "disputed"
)

// GeneDisease represents the gene_disease join table

type GeneDisease struct {
	GeneDiseaseID int             `db:"gene_disease_id" json:"gene_disease_id"`
	GeneID        int             `db:"gene_id" json:"gene_id"`
	DiseaseID     int             `db:"disease_id" json:"disease_id"`
	Association   AssociationType `db:"association_type" json:"association_type"`
	Evidence      EvidenceLevel   `db:"evidence_level" json:"evidence_level"`
	Source        *string         `db:"source" json:"source,omitempty"`
	SourceID      *string         `db:"source_id" json:"source_id,omitempty"`
	CreatedAt     time.Time       `db:"created_at" json:"created_at"`
}
