package models

import "time"

// PatientUpdate — payload PATCH /api/patients/{id}. Все поля опциональны.
type PatientUpdate struct {
	ExternalID           *string `json:"external_id,omitempty"`
	FirstName            *string `json:"first_name,omitempty"`
	LastName             *string `json:"last_name,omitempty"`
	DateOfBirth          *string `json:"date_of_birth,omitempty"`
	Sex                  *string `json:"sex,omitempty"`
	PhoneNumber          *string `json:"phone_number,omitempty"`
	Email                *string `json:"email,omitempty"`
	Address              *string `json:"address,omitempty"`
	PhenotypeDescription *string `json:"phenotype_description,omitempty"`
}

// VariantAnnotation — строка из public.variant_annotation (упрощённый view).
type VariantAnnotation struct {
	AnnotationID        int64    `json:"annotation_id"`
	VariantID           int      `json:"variant_id"`
	GeneID              *int     `json:"gene_id,omitempty"`
	Consequence         *string  `json:"consequence,omitempty"`
	Impact              *string  `json:"impact,omitempty"`
	TranscriptID        *string  `json:"transcript_id,omitempty"`
	HGVSc               *string  `json:"hgvsc,omitempty"`
	HGVSp               *string  `json:"hgvsp,omitempty"`
	ProteinPosition     *int     `json:"protein_position,omitempty"`
	AminoAcids          *string  `json:"amino_acids,omitempty"`
	Codons              *string  `json:"codons,omitempty"`
	GnomadAF            *float64 `json:"gnomad_af,omitempty"`
	GnomadAFPopmax      *float64 `json:"gnomad_af_popmax,omitempty"`
	SIFTScore           *float64 `json:"sift_score,omitempty"`
	SIFTPrediction      *string  `json:"sift_prediction,omitempty"`
	PolyphenScore       *float64 `json:"polyphen_score,omitempty"`
	PolyphenPrediction  *string  `json:"polyphen_prediction,omitempty"`
	CADDScore           *float64 `json:"cadd_score,omitempty"`
	RevelScore          *float64 `json:"revel_score,omitempty"`
	ClinvarSignificance *string  `json:"clinvar_significance,omitempty"`
	ClinvarID           *string  `json:"clinvar_id,omitempty"`
}

// Gene — public.gene.
type Gene struct {
	GeneID          int     `json:"gene_id"`
	GeneSymbol      *string `json:"gene_symbol,omitempty"`
	GeneName        *string `json:"gene_name,omitempty"`
	EnsemblGeneID   *string `json:"ensembl_gene_id,omitempty"`
	NCBIGeneID      *int    `json:"ncbi_gene_id,omitempty"`
	OMIMGeneID      *string `json:"omim_gene_id,omitempty"`
	Chromosome      *string `json:"chromosome,omitempty"`
	StartPosition   *int64  `json:"start_position,omitempty"`
	EndPosition     *int64  `json:"end_position,omitempty"`
	Strand          *string `json:"strand,omitempty"`
	GeneDescription *string `json:"gene_description,omitempty"`
}

// Phenotype — public.phenotype.
type Phenotype struct {
	PhenotypeID        int     `json:"phenotype_id"`
	PhenotypeName      string  `json:"phenotype_name"`
	OMIMPhenotypeID    *string `json:"omim_phenotype_id,omitempty"`
	OrphaCode          *string `json:"orpha_code,omitempty"`
	InheritancePattern *string `json:"inheritance_pattern,omitempty"`
	Description        *string `json:"description,omitempty"`
}

// VariantInterpretation — public.variant_interpretation.
type VariantInterpretation struct {
	InterpretationID    int64      `json:"interpretation_id"`
	PatientVariantID    int64      `json:"patient_variant_id"`
	UserID              int        `json:"user_id"`
	ACMGClassification  string     `json:"acmg_classification"`
	InterpretationText  *string    `json:"interpretation_text,omitempty"`
	DiseaseID           *int       `json:"disease_id,omitempty"`
	CreatedAt           *time.Time `json:"created_at,omitempty"`
	UpdatedAt           *time.Time `json:"updated_at,omitempty"`
}

// VariantDetails — агрегат для /api/variants/{id}.
type VariantDetails struct {
	Variant          Variant                 `json:"variant"`
	Annotations      []VariantAnnotation     `json:"annotations"`
	Genes            []Gene                  `json:"genes"`
	Phenotypes       []Phenotype             `json:"phenotypes"`
	Interpretations  []VariantInterpretation `json:"interpretations"`
	PatientCount     int                     `json:"patient_count"`
	EnrichmentStatus string                  `json:"enrichment_status"` // pending|ok|failed|skipped
}

// PatientVariantRich — расширенная строка для таблицы вариантов:
// patient_variant + аннотации + гены (агрегированные).
type PatientVariantRich struct {
	PatientVariant
	Annotations    []VariantAnnotation `json:"annotations,omitempty"`
	GeneSymbols    []string            `json:"gene_symbols,omitempty"`
	TopConsequence *string             `json:"top_consequence,omitempty"`
	TopImpact      *string             `json:"top_impact,omitempty"`
}

// VariantFilter — фильтры для GET /api/patients/{id}/variants.
type VariantFilter struct {
	Chrom       string
	VariantType string
	FilterEq    string
	MinQual     *float64
	Zygosity    string
	Q           string // полнотекстовый поиск: rs_id / chrom:pos / ген / HGVS / ClinVar
	Sort        string // например "chrom,position" или "-quality"
	Limit       int
	Offset      int
}

// VariantSearchFilter — фильтры для GET /api/variants (поиск по бд вариантов).
type VariantSearchFilter struct {
	RsID        string
	Gene        string
	Chrom       string
	Pos         *int64
	Ref         string
	Alt         string
	Build       string
	Q           string // глобальный поиск (приоритет: rs_id → gene_symbol → координаты)
	Limit       int
	Offset      int
}

// VariantSearchResult — строка результата GET /api/variants.
type VariantSearchResult struct {
	Variant             Variant  `json:"variant"`
	GeneSymbols         []string `json:"gene_symbols"`
	PatientCount        int      `json:"patient_count"` // у текущего пользователя
	TopImpact           *string  `json:"top_impact,omitempty"`
	ClinVarSignificance *string  `json:"clinvar_significance,omitempty"`
}

// SampleCreate — payload для multipart upload sample.
type SampleCreate struct {
	SampleName     string
	SampleType     string
	SequencingType string
	PanelName      *string
}

// PageResponse — унифицированный ответ списка.
type PageResponse struct {
	Items  any `json:"items"`
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}
