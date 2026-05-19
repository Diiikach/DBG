package models

import "time"

// Patient — пациент.
type Patient struct {
	PatientID             int        `json:"patient_id"`
	ExternalID            *string    `json:"external_id,omitempty"`
	FirstName             string     `json:"first_name"`
	LastName              string     `json:"last_name"`
	DateOfBirth           *string    `json:"date_of_birth,omitempty"` // YYYY-MM-DD
	Sex                   *string    `json:"sex,omitempty"`           // male/female/other
	PhoneNumber           *string    `json:"phone_number,omitempty"`
	Email                 *string    `json:"email,omitempty"`
	Address               *string    `json:"address,omitempty"`
	PhenotypeDescription  *string    `json:"phenotype_description,omitempty"`
	CreatedAt             *time.Time `json:"created_at,omitempty"`
	UpdatedAt             *time.Time `json:"updated_at,omitempty"`
}

// PatientCreate — payload для создания пациента.
type PatientCreate struct {
	ExternalID            *string `json:"external_id"`
	FirstName             string  `json:"first_name"`
	LastName              string  `json:"last_name"`
	DateOfBirth           *string `json:"date_of_birth"`
	Sex                   *string `json:"sex"`
	PhoneNumber           *string `json:"phone_number"`
	Email                 *string `json:"email"`
	Address               *string `json:"address"`
	PhenotypeDescription  *string `json:"phenotype_description"`
}

// Variant — найденный вариант (SNV/INDEL/...).
type Variant struct {
	VariantID   int     `json:"variant_id"`
	Chromosome  string  `json:"chromosome"`
	Position    int64   `json:"position"`
	Reference   string  `json:"reference"`
	Alternate   string  `json:"alternate"`
	RsID        *string `json:"rs_id,omitempty"`
	GenomeBuild *string `json:"genome_build,omitempty"`
	VariantType *string `json:"variant_type,omitempty"`
}

// PatientVariant — вариант, обнаруженный у пациента (срез join).
type PatientVariant struct {
	PatientVariantID int64    `json:"patient_variant_id"`
	PatientID        int      `json:"patient_id"`
	SampleID         *int     `json:"sample_id,omitempty"`
	Variant          Variant  `json:"variant"`
	Zygosity         *string  `json:"zygosity,omitempty"`
	Quality          *float64 `json:"quality,omitempty"`
	ReadDepth        *int     `json:"read_depth,omitempty"`
	AlleleDepthRef   *int     `json:"allele_depth_ref,omitempty"`
	AlleleDepthAlt   *int     `json:"allele_depth_alt,omitempty"`
	GenotypeQuality  *int     `json:"genotype_quality,omitempty"`
	FilterStatus     *string  `json:"filter_status,omitempty"`
	DetectedAt       *time.Time `json:"detected_at,omitempty"`
}

// Sample — образец секвенирования.
type Sample struct {
	SampleID           int        `json:"sample_id"`
	PatientID          int        `json:"patient_id"`
	SampleName         string     `json:"sample_name"`
	SampleType         string     `json:"sample_type"`
	SequencingType     string     `json:"sequencing_type"`
	PanelName          *string    `json:"panel_name,omitempty"`
	SequencingPlatform *string    `json:"sequencing_platform,omitempty"`
	SequencingDate     *string    `json:"sequencing_date,omitempty"`
	MeanCoverage       *float64   `json:"mean_coverage,omitempty"`
	VCFFilePath        *string    `json:"vcf_file_path,omitempty"`
	ProcessingStatus   string     `json:"processing_status"`
	FailureReason      *string    `json:"failure_reason,omitempty"`
	CreatedAt          *time.Time `json:"created_at,omitempty"`
}

// SampleWithPatient — Sample c полями пациента для глобального списка
// загрузок на дашборде (GET /api/samples).
type SampleWithPatient struct {
	Sample
	PatientExternalID *string `json:"patient_external_id,omitempty"`
	PatientFirstName  string  `json:"patient_first_name"`
	PatientLastName   string  `json:"patient_last_name"`
}

// VCFRecord — внутреннее представление одной строки VCF после парсинга.
// Также используется как единое DTO для ручного ввода и CSV-импорта.
type VCFRecord struct {
	Chromosome string
	Position   int64
	Reference  string
	Alternate  string
	Quality    *float64
	Filter     string
	// Опциональные поля (заполняются для manual/CSV; для пайплайна обычно nil).
	RsID        *string
	GenomeBuild *string // если nil — используется build из cfg
	VariantType *string // если nil — будет classifyVariant(ref,alt)
	// Поля из FORMAT для одного семпла:
	Zygosity        *string
	ReadDepth       *int
	AlleleDepthRef  *int
	AlleleDepthAlt  *int
	GenotypeQuality *int
}

// AlignRequest — запрос на выравнивание и поиск вариантов.
type AlignRequest struct {
	PatientID  int      `json:"patient_id"`
	SampleName string   `json:"sample_name"`
	Reads      []string `json:"reads"` // последовательности нуклеотидов (FASTA-стиль)
}

// AlignResponse — ответ после прогонки пайплайна.
type AlignResponse struct {
	SampleID  int        `json:"sample_id"`
	PatientID int        `json:"patient_id"`
	Variants  []VCFRecord `json:"variants"`
	VCFPath   string     `json:"vcf_path"`
	Status    string     `json:"status"`
}
