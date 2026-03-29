package models

import "time"

type ExternalVariantData struct {
	ExternalID int64     `db:"external_id" json:"external_id"`
	VariantID  *int      `db:"variant_id" json:"variant_id,omitempty"`
	RSID       *string   `db:"rs_id" json:"rs_id,omitempty"`
	Source     string    `db:"source" json:"source"`
	Payload    []byte    `db:"payload" json:"-"`
	FetchedAt  time.Time `db:"fetched_at" json:"fetched_at"`
}
