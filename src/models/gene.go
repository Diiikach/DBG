package models

// Direction corresponds to SQL enum direction ('+' or '-')
type Direction string

const (
	DirectionPlus  Direction = "+"
	DirectionMinus Direction = "-"
)

// Gene represents the gene table

type Gene struct {
	GeneID          int        `db:"gene_id" json:"gene_id"`
	GeneSymbol      *string    `db:"gene_symbol" json:"gene_symbol,omitempty"`
	GeneName        *string    `db:"gene_name" json:"gene_name,omitempty"`
	EnsemblGeneID   *string    `db:"ensembl_gene_id" json:"ensembl_gene_id,omitempty"`
	NCBIGeneID      *int       `db:"ncbi_gene_id" json:"ncbi_gene_id,omitempty"`
	OMIMGeneID      *string    `db:"omim_gene_id" json:"omim_gene_id,omitempty"`
	Chromosome      *string    `db:"chromosome" json:"chromosome,omitempty"`
	StartPosition   *int64     `db:"start_position" json:"start_position,omitempty"`
	EndPosition     *int64     `db:"end_position" json:"end_position,omitempty"`
	Strand          *Direction `db:"strand" json:"strand,omitempty"`
	GeneDescription *string    `db:"gene_description" json:"gene_description,omitempty"`
}
