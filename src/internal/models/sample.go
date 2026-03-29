package models

import "time"

// SeqType corresponds to SQL enum seq_type
// Values: "WGS", "WES", "PANEL", "RNA-SEQ", "OTHER".
type SeqType string

const (
	SeqTypeWGS    SeqType = "WGS"
	SeqTypeWES    SeqType = "WES"
	SeqTypePanel  SeqType = "PANEL"
	SeqTypeRNASeq SeqType = "RNA-SEQ"
	SeqTypeOther  SeqType = "OTHER"
)

// Status corresponds to SQL enum status for sample processing
// Values: 'uploaded','processing','annotated','completed','failed'
type SampleStatus string

const (
	SampleStatusUploaded   SampleStatus = "uploaded"
	SampleStatusProcessing SampleStatus = "processing"
	SampleStatusAnnotated  SampleStatus = "annotated"
	SampleStatusCompleted  SampleStatus = "completed"
	SampleStatusFailed     SampleStatus = "failed"
)

// Sample represents the sample table.
// Foreign keys: patient_id -> patient.patient_id

type Sample struct {
	SampleID           int          `db:"sample_id" json:"sample_id"`
	PatientID          int          `db:"patient_id" json:"patient_id"`
	SampleName         string       `db:"sample_name" json:"sample_name"`
	SampleType         string       `db:"sample_type" json:"sample_type"`
	SequencingType     SeqType      `db:"sequencing_type" json:"sequencing_type"`
	PanelName          *string      `db:"panel_name" json:"panel_name,omitempty"`
	SequencingPlatform *string      `db:"sequencing_platform" json:"sequencing_platform,omitempty"`
	SequencingDate     *time.Time   `db:"sequencing_date" json:"sequencing_date,omitempty"`
	MeanCoverage       *float64     `db:"mean_coverage" json:"mean_coverage,omitempty"`
	VCFFilePath        *string      `db:"vcf_file_path" json:"vcf_file_path,omitempty"`
	ProcessingStatus   SampleStatus `db:"processing_status" json:"processing_status"`
	CreatedAt          time.Time    `db:"created_at" json:"created_at"`
}
