package httpapi

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type healthResponse struct {
	Status string `json:"status"`
	DB     string `json:"db"`
}

type errorResponse struct {
	Error       string            `json:"error"`
	FieldErrors map[string]string `json:"field_errors,omitempty"`
}

func (a *App) healthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}

	dbStatus := "unknown"
	if a.db != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := a.db.PingContext(ctx); err != nil {
			dbStatus = "down"
		} else {
			dbStatus = "up"
		}
	}

	writeJSON(w, http.StatusOK, healthResponse{Status: "ok", DB: dbStatus})
}

// Patients

type patientInput struct {
	ExternalID           string  `json:"external_id"`
	FirstName            string  `json:"first_name"`
	LastName             string  `json:"last_name"`
	DateOfBirth          string  `json:"date_of_birth"`
	Sex                  string  `json:"sex"`
	PhoneNumber          *string `json:"phone_number"`
	Email                *string `json:"email"`
	Address              *string `json:"address"`
	PhenotypeDescription *string `json:"phenotype_description"`
}

type patientOutput struct {
	PatientID            int       `json:"patient_id"`
	ExternalID           string    `json:"external_id"`
	FirstName            string    `json:"first_name"`
	LastName             string    `json:"last_name"`
	DateOfBirth          time.Time `json:"date_of_birth"`
	Sex                  string    `json:"sex"`
	PhoneNumber          *string   `json:"phone_number,omitempty"`
	Email                *string   `json:"email,omitempty"`
	Address              *string   `json:"address,omitempty"`
	PhenotypeDescription *string   `json:"phenotype_description,omitempty"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

func (a *App) patientsHandler(w http.ResponseWriter, r *http.Request) {
	if a.db == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "database not configured"})
		return
	}

	switch r.Method {
	case http.MethodPost:
		a.createPatient(w, r)
	case http.MethodGet:
		a.listPatients(w, r)
	default:
		methodNotAllowed(w, http.MethodPost, http.MethodGet)
	}
}

func (a *App) createPatient(w http.ResponseWriter, r *http.Request) {
	var in patientInput
	if err := readJSON(r.Body, &in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	fieldErrs := map[string]string{}
	if strings.TrimSpace(in.ExternalID) == "" {
		fieldErrs["external_id"] = "обязательно"
	}
	if strings.TrimSpace(in.FirstName) == "" {
		fieldErrs["first_name"] = "обязательно"
	}
	if strings.TrimSpace(in.LastName) == "" {
		fieldErrs["last_name"] = "обязательно"
	}
	if in.Sex != "" && !isSexValid(in.Sex) {
		fieldErrs["sex"] = "допустимо: male, female, other"
	}
	if in.Email != nil && *in.Email != "" && !isEmailValid(*in.Email) {
		fieldErrs["email"] = "некорректный email"
	}
	if len(fieldErrs) > 0 {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "валидация полей", FieldErrors: fieldErrs})
		return
	}
	dob, err := time.Parse("2006-01-02", in.DateOfBirth)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "валидация полей", FieldErrors: map[string]string{"date_of_birth": "формат YYYY-MM-DD"}})
		return
	}

	userID := userIDFromContext(r.Context())
	if userID == 0 {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "unauthorized"})
		return
	}

	var out patientOutput
	query := `
		INSERT INTO patient (external_id, first_name, last_name, date_of_birth, sex, phone_number, email, address, phenotype_description, owner_user_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		RETURNING patient_id, external_id, first_name, last_name, date_of_birth, sex, phone_number, email, address, phenotype_description, created_at, updated_at`
	err = a.db.QueryRow(query,
		in.ExternalID, in.FirstName, in.LastName, dob, in.Sex, in.PhoneNumber, in.Email, in.Address, in.PhenotypeDescription, userID,
	).Scan(&out.PatientID, &out.ExternalID, &out.FirstName, &out.LastName, &out.DateOfBirth, &out.Sex, &out.PhoneNumber, &out.Email, &out.Address, &out.PhenotypeDescription, &out.CreatedAt, &out.UpdatedAt)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, out)
}

func (a *App) listPatients(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromContext(r.Context())
	if userID == 0 {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "unauthorized"})
		return
	}
	q := r.URL.Query()
	if idStr := q.Get("patient_id"); idStr != "" {
		id, err := strconv.Atoi(idStr)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid patient_id"})
			return
		}
		a.getPatientByID(w, id, userID)
		return
	}
	if externalID := q.Get("external_id"); externalID != "" {
		a.getPatientByExternalID(w, externalID, userID)
		return
	}

	limit, offset := parseLimitOffset(q)
	rows, err := a.db.Query(`
		SELECT patient_id, external_id, first_name, last_name, date_of_birth, sex, phone_number, email, address, phenotype_description, created_at, updated_at
		FROM patient
		WHERE owner_user_id = $1
		ORDER BY patient_id
		LIMIT $2 OFFSET $3`, userID, limit, offset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	defer rows.Close()

	var out []patientOutput
	for rows.Next() {
		var p patientOutput
		if err := rows.Scan(&p.PatientID, &p.ExternalID, &p.FirstName, &p.LastName, &p.DateOfBirth, &p.Sex, &p.PhoneNumber, &p.Email, &p.Address, &p.PhenotypeDescription, &p.CreatedAt, &p.UpdatedAt); err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
			return
		}
		out = append(out, p)
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *App) getPatientByID(w http.ResponseWriter, id int, userID int) {
	var p patientOutput
	err := a.db.QueryRow(`
		SELECT patient_id, external_id, first_name, last_name, date_of_birth, sex, phone_number, email, address, phenotype_description, created_at, updated_at
		FROM patient WHERE patient_id = $1 AND owner_user_id = $2`, id, userID).
		Scan(&p.PatientID, &p.ExternalID, &p.FirstName, &p.LastName, &p.DateOfBirth, &p.Sex, &p.PhoneNumber, &p.Email, &p.Address, &p.PhenotypeDescription, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "patient not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (a *App) getPatientByExternalID(w http.ResponseWriter, externalID string, userID int) {
	var p patientOutput
	err := a.db.QueryRow(`
		SELECT patient_id, external_id, first_name, last_name, date_of_birth, sex, phone_number, email, address, phenotype_description, created_at, updated_at
		FROM patient WHERE external_id = $1 AND owner_user_id = $2`, externalID, userID).
		Scan(&p.PatientID, &p.ExternalID, &p.FirstName, &p.LastName, &p.DateOfBirth, &p.Sex, &p.PhoneNumber, &p.Email, &p.Address, &p.PhenotypeDescription, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "patient not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// Variants

type variantInput struct {
	Chromosome      string  `json:"chromosome"`
	Position        int64   `json:"position"`
	ReferenceAllele string  `json:"reference_allele"`
	AlternateAllele string  `json:"alternate_allele"`
	RSID            *string `json:"rs_id"`
	GenomeBuild     string  `json:"genome_build"`
	VariantType     string  `json:"variant_type"`
	PatientID       *int    `json:"patient_id"`
}

type variantOutput struct {
	VariantID       int        `json:"variant_id"`
	Chromosome      string     `json:"chromosome"`
	Position        int64      `json:"position"`
	ReferenceAllele string     `json:"reference_allele"`
	AlternateAllele string     `json:"alternate_allele"`
	RSID            *string    `json:"rs_id,omitempty"`
	GenomeBuild     string     `json:"genome_build"`
	VariantType     string     `json:"variant_type"`
	CreatedAt       *time.Time `json:"created_at,omitempty"`
}

func (a *App) variantsHandler(w http.ResponseWriter, r *http.Request) {
	if a.db == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "database not configured"})
		return
	}

	switch r.Method {
	case http.MethodPost:
		a.createVariant(w, r)
	case http.MethodGet:
		a.listVariants(w, r)
	default:
		methodNotAllowed(w, http.MethodPost, http.MethodGet)
	}
}

func (a *App) createVariant(w http.ResponseWriter, r *http.Request) {
	var in variantInput
	if err := readJSON(r.Body, &in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	fieldErrs := map[string]string{}
	if strings.TrimSpace(in.Chromosome) == "" {
		fieldErrs["chromosome"] = "обязательно"
	}
	if in.Position <= 0 {
		fieldErrs["position"] = "должно быть > 0"
	}
	if strings.TrimSpace(in.ReferenceAllele) == "" {
		fieldErrs["reference_allele"] = "обязательно"
	}
	if strings.TrimSpace(in.AlternateAllele) == "" {
		fieldErrs["alternate_allele"] = "обязательно"
	}
	if in.GenomeBuild == "" || !isGenomeBuildValid(in.GenomeBuild) {
		fieldErrs["genome_build"] = "допустимо: GRCh37, GRCh38"
	}
	if in.VariantType == "" || !isVariantTypeValid(in.VariantType) {
		fieldErrs["variant_type"] = "допустимо: SNV, INS, DEL, INDEL, MNV, CNV, SV, OTHER"
	}
	if !isAlleleValid(in.ReferenceAllele) {
		fieldErrs["reference_allele"] = "должно содержать только A,C,G,T,N или быть '-'\n"
	}
	if !isAlleleValid(in.AlternateAllele) {
		fieldErrs["alternate_allele"] = "должно содержать только A,C,G,T,N или быть '-'\n"
	}
	if len(fieldErrs) > 0 {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "валидация полей", FieldErrors: fieldErrs})
		return
	}

	var out variantOutput
	query := `
		INSERT INTO variant (chromosome, position, reference_allele, alternate_allele, rs_id, genome_build, variant_type)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING variant_id, chromosome, position, reference_allele, alternate_allele, rs_id, genome_build, variant_type, created_at`
	err := a.db.QueryRow(query, in.Chromosome, in.Position, in.ReferenceAllele, in.AlternateAllele, in.RSID, in.GenomeBuild, in.VariantType).
		Scan(&out.VariantID, &out.Chromosome, &out.Position, &out.ReferenceAllele, &out.AlternateAllele, &out.RSID, &out.GenomeBuild, &out.VariantType, &out.CreatedAt)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	if in.PatientID != nil && *in.PatientID > 0 {
		userID := userIDFromContext(r.Context())
		if userID == 0 || !a.patientOwnedByUser(*in.PatientID, userID) {
			writeJSON(w, http.StatusForbidden, errorResponse{Error: "forbidden"})
			return
		}
		_, _ = a.db.Exec(`
			INSERT INTO patient_variant (patient_id, variant_id)
			VALUES ($1,$2)
			ON CONFLICT (patient_id, variant_id) DO NOTHING`, *in.PatientID, out.VariantID)
	}
	writeJSON(w, http.StatusCreated, out)
}

func (a *App) listVariants(w http.ResponseWriter, r *http.Request) {
	userID := userIDFromContext(r.Context())
	if userID == 0 {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "unauthorized"})
		return
	}
	q := r.URL.Query()
	if rsid := q.Get("rs_id"); rsid != "" {
		a.getVariantsByRSID(w, rsid, userID)
		return
	}
	if q.Get("chromosome") != "" && q.Get("position") != "" && q.Get("reference_allele") != "" && q.Get("alternate_allele") != "" {
		a.getVariantsByLocus(w, q.Get("chromosome"), q.Get("position"), q.Get("reference_allele"), q.Get("alternate_allele"), userID)
		return
	}

	limit, offset := parseLimitOffset(q)
	rows, err := a.db.Query(`
		SELECT v.variant_id, v.chromosome, v.position, v.reference_allele, v.alternate_allele, v.rs_id, v.genome_build, v.variant_type, v.created_at
		FROM variant v
		ORDER BY v.variant_id
		LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	defer rows.Close()

	var out []variantOutput
	for rows.Next() {
		var v variantOutput
		if err := rows.Scan(&v.VariantID, &v.Chromosome, &v.Position, &v.ReferenceAllele, &v.AlternateAllele, &v.RSID, &v.GenomeBuild, &v.VariantType, &v.CreatedAt); err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
			return
		}
		out = append(out, v)
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *App) getVariantsByRSID(w http.ResponseWriter, rsid string, userID int) {
	_ = userID
	rows, err := a.db.Query(`
		SELECT v.variant_id, v.chromosome, v.position, v.reference_allele, v.alternate_allele, v.rs_id, v.genome_build, v.variant_type, v.created_at
		FROM variant v
		WHERE v.rs_id = $1`, rsid)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	defer rows.Close()

	var out []variantOutput
	for rows.Next() {
		var v variantOutput
		if err := rows.Scan(&v.VariantID, &v.Chromosome, &v.Position, &v.ReferenceAllele, &v.AlternateAllele, &v.RSID, &v.GenomeBuild, &v.VariantType, &v.CreatedAt); err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
			return
		}
		out = append(out, v)
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *App) getVariantsByLocus(w http.ResponseWriter, chr, posStr, ref, alt string, userID int) {
	pos, err := strconv.ParseInt(posStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid position"})
		return
	}
	_ = userID
	rows, err := a.db.Query(`
		SELECT v.variant_id, v.chromosome, v.position, v.reference_allele, v.alternate_allele, v.rs_id, v.genome_build, v.variant_type, v.created_at
		FROM variant v
		WHERE v.chromosome = $1 AND v.position = $2 AND v.reference_allele = $3 AND v.alternate_allele = $4`,
		chr, pos, ref, alt)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	defer rows.Close()

	var out []variantOutput
	for rows.Next() {
		var v variantOutput
		if err := rows.Scan(&v.VariantID, &v.Chromosome, &v.Position, &v.ReferenceAllele, &v.AlternateAllele, &v.RSID, &v.GenomeBuild, &v.VariantType, &v.CreatedAt); err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
			return
		}
		out = append(out, v)
	}
	writeJSON(w, http.StatusOK, out)
}

// Patient-variant links

type patientVariantInput struct {
	PatientID       int      `json:"patient_id"`
	VariantID       int      `json:"variant_id"`
	SampleID        *int     `json:"sample_id"`
	Zygosity        *string  `json:"zygosity"`
	Quality         *float64 `json:"quality"`
	ReadDepth       *int     `json:"read_depth"`
	AlleleDepthRef  *int     `json:"allele_depth_ref"`
	AlleleDepthAlt  *int     `json:"allele_depth_alt"`
	GenotypeQuality *int     `json:"genotype_quality"`
	FilterStatus    *string  `json:"filter_status"`
}

type patientVariantOutput struct {
	PatientVariantID int64     `json:"patient_variant_id"`
	CreatedAt        time.Time `json:"created_at"`
}

func (a *App) patientVariantsHandler(w http.ResponseWriter, r *http.Request) {
	if a.db == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "database not configured"})
		return
	}
	switch r.Method {
	case http.MethodPost:
		var in patientVariantInput
		if err := readJSON(r.Body, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
			return
		}
		fieldErrs := map[string]string{}
		if in.PatientID == 0 {
			fieldErrs["patient_id"] = "обязательно"
		}
		if in.VariantID == 0 {
			fieldErrs["variant_id"] = "обязательно"
		}
		if in.Zygosity != nil && !isZygosityValid(*in.Zygosity) {
			fieldErrs["zygosity"] = "допустимо: heterozygous, homozygous, hemizygous"
		}
		if len(fieldErrs) > 0 {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "валидация полей", FieldErrors: fieldErrs})
			return
		}

		userID := userIDFromContext(r.Context())
		if userID == 0 || !a.patientOwnedByUser(in.PatientID, userID) {
			writeJSON(w, http.StatusForbidden, errorResponse{Error: "forbidden"})
			return
		}

		var out patientVariantOutput
		err := a.db.QueryRow(`
			INSERT INTO patient_variant (patient_id, variant_id, sample_id, zygosity, quality, read_depth, allele_depth_ref, allele_depth_alt, genotype_quality, filter_status)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
			RETURNING patient_variant_id, created_at`,
			in.PatientID, in.VariantID, in.SampleID, in.Zygosity, in.Quality, in.ReadDepth, in.AlleleDepthRef, in.AlleleDepthAlt, in.GenotypeQuality, in.FilterStatus).
			Scan(&out.PatientVariantID, &out.CreatedAt)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, out)
	case http.MethodGet:
		a.listPatientVariants(w, r)
	default:
		methodNotAllowed(w, http.MethodPost, http.MethodGet)
	}
}

// Variant annotations (manual upsert)

type variantAnnotationInput struct {
	VariantID           int      `json:"variant_id"`
	GeneID              *int     `json:"gene_id"`
	Consequence         *string  `json:"consequence"`
	Impact              *string  `json:"impact"`
	TranscriptID        *string  `json:"transcript_id"`
	Hgvsc               *string  `json:"hgvsc"`
	Hgvsp               *string  `json:"hgvsp"`
	ProteinPosition     *int     `json:"protein_position"`
	AminoAcids          *string  `json:"amino_acids"`
	Codons              *string  `json:"codons"`
	GnomadAF            *float64 `json:"gnomad_af"`
	GnomadAFPopmax      *float64 `json:"gnomad_af_popmax"`
	SiftScore           *float64 `json:"sift_score"`
	SiftPrediction      *string  `json:"sift_prediction"`
	PolyphenScore       *float64 `json:"polyphen_score"`
	PolyphenPrediction  *string  `json:"polyphen_prediction"`
	CaddScore           *float64 `json:"cadd_score"`
	RevelScore          *float64 `json:"revel_score"`
	ClinvarSignificance *string  `json:"clinvar_significance"`
	ClinvarID           *string  `json:"clinvar_id"`
}

func (a *App) variantAnnotationsHandler(w http.ResponseWriter, r *http.Request) {
	if a.db == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "database not configured"})
		return
	}
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}

	var in variantAnnotationInput
	if err := readJSON(r.Body, &in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	fieldErrs := map[string]string{}
	if in.VariantID == 0 {
		fieldErrs["variant_id"] = "обязательно"
	}
	if in.Impact != nil && !isImpactValid(*in.Impact) {
		fieldErrs["impact"] = "допустимо: HIGH, MODERATE, LOW, MODIFIER"
	}
	if in.SiftPrediction != nil && !isSiftValid(*in.SiftPrediction) {
		fieldErrs["sift_prediction"] = "допустимо: tolerated, deleterious"
	}
	if in.PolyphenPrediction != nil && !isPolyphenValid(*in.PolyphenPrediction) {
		fieldErrs["polyphen_prediction"] = "допустимо: benign, possibly_damaging, probably_damaging"
	}
	if in.ClinvarSignificance != nil && !isClinvarValid(*in.ClinvarSignificance) {
		fieldErrs["clinvar_significance"] = "некорректное значение ClinVar"
	}
	if len(fieldErrs) > 0 {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "валидация полей", FieldErrors: fieldErrs})
		return
	}

	_, err := a.db.Exec(`
		INSERT INTO variant_annotation (
			variant_id, gene_id, consequence, impact, transcript_id, hgvsc, hgvsp, protein_position,
			amino_acids, codons, gnomad_af, gnomad_af_popmax, sift_score, sift_prediction,
			polyphen_score, polyphen_prediction, cadd_score, revel_score, clinvar_significance, clinvar_id
		) VALUES (
			$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20
		)
		ON CONFLICT (variant_id) DO UPDATE SET
			gene_id = EXCLUDED.gene_id,
			consequence = EXCLUDED.consequence,
			impact = EXCLUDED.impact,
			transcript_id = EXCLUDED.transcript_id,
			hgvsc = EXCLUDED.hgvsc,
			hgvsp = EXCLUDED.hgvsp,
			protein_position = EXCLUDED.protein_position,
			amino_acids = EXCLUDED.amino_acids,
			codons = EXCLUDED.codons,
			gnomad_af = EXCLUDED.gnomad_af,
			gnomad_af_popmax = EXCLUDED.gnomad_af_popmax,
			sift_score = EXCLUDED.sift_score,
			sift_prediction = EXCLUDED.sift_prediction,
			polyphen_score = EXCLUDED.polyphen_score,
			polyphen_prediction = EXCLUDED.polyphen_prediction,
			cadd_score = EXCLUDED.cadd_score,
			revel_score = EXCLUDED.revel_score,
			clinvar_significance = EXCLUDED.clinvar_significance,
			clinvar_id = EXCLUDED.clinvar_id`,
		in.VariantID, in.GeneID, in.Consequence, in.Impact, in.TranscriptID, in.Hgvsc, in.Hgvsp, in.ProteinPosition,
		in.AminoAcids, in.Codons, in.GnomadAF, in.GnomadAFPopmax, in.SiftScore, in.SiftPrediction,
		in.PolyphenScore, in.PolyphenPrediction, in.CaddScore, in.RevelScore, in.ClinvarSignificance, in.ClinvarID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Similar variants search

func (a *App) similarVariantsHandler(w http.ResponseWriter, r *http.Request) {
	if a.db == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "database not configured"})
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}

	q := r.URL.Query()
	by := q.Get("by")
	userID := userIDFromContext(r.Context())
	if userID == 0 {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "unauthorized"})
		return
	}
	switch by {
	case "rsid":
		rsid := q.Get("rs_id")
		if rsid == "" {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "rs_id is required"})
			return
		}
		a.similarByRSID(w, rsid, userID)
	case "gene":
		geneID := q.Get("gene_id")
		geneSymbol := q.Get("gene_symbol")
		if geneID == "" && geneSymbol == "" {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "gene_id or gene_symbol is required"})
			return
		}
		a.similarByGene(w, geneID, geneSymbol, userID)
	case "locus":
		chr := q.Get("chromosome")
		pos := q.Get("position")
		ref := q.Get("reference_allele")
		alt := q.Get("alternate_allele")
		if chr == "" || pos == "" || ref == "" || alt == "" {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "chromosome, position, reference_allele, alternate_allele are required"})
			return
		}
		a.similarByLocus(w, chr, pos, ref, alt, userID)
	default:
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "by must be one of: rsid, gene, locus"})
	}
}

type similarResult struct {
	VariantID   int     `json:"variant_id"`
	Chromosome  string  `json:"chromosome"`
	Position    int64   `json:"position"`
	Reference   string  `json:"reference_allele"`
	Alternate   string  `json:"alternate_allele"`
	RSID        *string `json:"rs_id,omitempty"`
	GeneSymbol  *string `json:"gene_symbol,omitempty"`
	ClinvarSign *string `json:"clinvar_significance,omitempty"`
}

func (a *App) similarByRSID(w http.ResponseWriter, rsid string, userID int) {
	_ = userID
	rows, err := a.db.Query(`
		SELECT DISTINCT v.variant_id, v.chromosome, v.position,
			v.reference_allele, v.alternate_allele, v.rs_id, g.gene_symbol, va.clinvar_significance
		FROM variant v
		LEFT JOIN variant_annotation va ON va.variant_id = v.variant_id
		LEFT JOIN gene g ON g.gene_id = va.gene_id
		WHERE v.rs_id = $1`, rsid)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	defer rows.Close()

	var out []similarResult
	for rows.Next() {
		var r similarResult
		if err := rows.Scan(&r.VariantID, &r.Chromosome, &r.Position, &r.Reference, &r.Alternate, &r.RSID, &r.GeneSymbol, &r.ClinvarSign); err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
			return
		}
		out = append(out, r)
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *App) similarByGene(w http.ResponseWriter, geneID, geneSymbol string, userID int) {
	var rows *sql.Rows
	var err error
	_ = userID
	if geneID != "" {
		id, convErr := strconv.Atoi(geneID)
		if convErr != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid gene_id"})
			return
		}
		rows, err = a.db.Query(`
			SELECT DISTINCT v.variant_id, v.chromosome, v.position,
				v.reference_allele, v.alternate_allele, v.rs_id, g.gene_symbol, va.clinvar_significance
			FROM gene g
			JOIN variant_annotation va ON va.gene_id = g.gene_id
			JOIN variant v ON v.variant_id = va.variant_id
			WHERE g.gene_id = $1`, id)
	} else {
		rows, err = a.db.Query(`
			SELECT DISTINCT v.variant_id, v.chromosome, v.position,
				v.reference_allele, v.alternate_allele, v.rs_id, g.gene_symbol, va.clinvar_significance
			FROM gene g
			JOIN variant_annotation va ON va.gene_id = g.gene_id
			JOIN variant v ON v.variant_id = va.variant_id
			WHERE g.gene_symbol = $1`, geneSymbol)
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	defer rows.Close()

	var out []similarResult
	for rows.Next() {
		var r similarResult
		if err := rows.Scan(&r.VariantID, &r.Chromosome, &r.Position, &r.Reference, &r.Alternate, &r.RSID, &r.GeneSymbol, &r.ClinvarSign); err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
			return
		}
		out = append(out, r)
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *App) similarByLocus(w http.ResponseWriter, chr, posStr, ref, alt string, userID int) {
	pos, err := strconv.ParseInt(posStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid position"})
		return
	}
	_ = userID
	rows, err := a.db.Query(`
		SELECT DISTINCT v.variant_id, v.chromosome, v.position,
			v.reference_allele, v.alternate_allele, v.rs_id, g.gene_symbol, va.clinvar_significance
		FROM variant v
		LEFT JOIN variant_annotation va ON va.variant_id = v.variant_id
		LEFT JOIN gene g ON g.gene_id = va.gene_id
		WHERE v.chromosome = $1 AND v.position = $2 AND v.reference_allele = $3 AND v.alternate_allele = $4`,
		chr, pos, ref, alt)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	defer rows.Close()

	var out []similarResult
	for rows.Next() {
		var r similarResult
		if err := rows.Scan(&r.VariantID, &r.Chromosome, &r.Position, &r.Reference, &r.Alternate, &r.RSID, &r.GeneSymbol, &r.ClinvarSign); err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
			return
		}
		out = append(out, r)
	}
	writeJSON(w, http.StatusOK, out)
}

// External enrichment

type enrichRequest struct {
	VariantID       *int   `json:"variant_id"`
	RSID            string `json:"rs_id"`
	Chromosome      string `json:"chromosome"`
	Position        int64  `json:"position"`
	ReferenceAllele string `json:"reference_allele"`
	AlternateAllele string `json:"alternate_allele"`
	GenomeBuild     string `json:"genome_build"`
}

type enrichResponse struct {
	ExternalID int64           `json:"external_id"`
	Source     string          `json:"source"`
	FetchedAt  time.Time       `json:"fetched_at"`
	Payload    json.RawMessage `json:"payload"`
}

func (a *App) enrichVariantHandler(w http.ResponseWriter, r *http.Request) {
	if a.db == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "database not configured"})
		return
	}
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}

	var in enrichRequest
	if err := readJSON(r.Body, &in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	if in.VariantID != nil {
		userID := userIDFromContext(r.Context())
		if userID == 0 || !a.variantOwnedByUser(*in.VariantID, userID) {
			writeJSON(w, http.StatusForbidden, errorResponse{Error: "forbidden"})
			return
		}
		var v variantOutput
		err := a.db.QueryRow(`
			SELECT variant_id, chromosome, position, reference_allele, alternate_allele, rs_id, genome_build, variant_type, created_at
			FROM variant WHERE variant_id = $1`, *in.VariantID).
			Scan(&v.VariantID, &v.Chromosome, &v.Position, &v.ReferenceAllele, &v.AlternateAllele, &v.RSID, &v.GenomeBuild, &v.VariantType, &v.CreatedAt)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "variant not found"})
			return
		}
		in.RSID = ""
		if v.RSID != nil {
			in.RSID = *v.RSID
		}
		in.Chromosome = v.Chromosome
		in.Position = v.Position
		in.ReferenceAllele = v.ReferenceAllele
		in.AlternateAllele = v.AlternateAllele
		in.GenomeBuild = v.GenomeBuild
	}

	query, assembly, rsid := buildMyVariantQuery(in)
	if query == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "rs_id or SNV locus required for enrichment"})
		return
	}

	payload, err := a.fetchMyVariant(query, assembly)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: err.Error()})
		return
	}

	if !json.Valid(payload) {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "invalid JSON from external source"})
		return
	}

	var externalID int64
	var fetchedAt time.Time
	err = a.db.QueryRow(`
		INSERT INTO external_variant_data (variant_id, rs_id, source, payload)
		VALUES ($1,$2,$3,$4::jsonb)
		RETURNING external_id, fetched_at`,
		in.VariantID, rsid, "myvariant", payload).Scan(&externalID, &fetchedAt)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	a.tryAttachDisease(in.VariantID, payload)

	writeJSON(w, http.StatusOK, enrichResponse{
		ExternalID: externalID,
		Source:     "myvariant",
		FetchedAt:  fetchedAt,
		Payload:    json.RawMessage(payload),
	})
}

func buildMyVariantQuery(in enrichRequest) (query string, assembly string, rsid string) {
	rsid = strings.TrimSpace(in.RSID)
	if rsid != "" {
		return rsid, "", rsid
	}
	if in.Chromosome == "" || in.Position == 0 || in.ReferenceAllele == "" || in.AlternateAllele == "" {
		return "", "", ""
	}
	if len(in.ReferenceAllele) != 1 || len(in.AlternateAllele) != 1 {
		return "", "", ""
	}
	query = fmt.Sprintf("chr%s:g.%d%s>%s", in.Chromosome, in.Position, in.ReferenceAllele, in.AlternateAllele)
	if strings.EqualFold(in.GenomeBuild, "GRCh38") {
		assembly = "hg38"
	}
	return query, assembly, rsid
}

func (a *App) fetchMyVariant(query, assembly string) ([]byte, error) {
	u, _ := url.Parse("https://myvariant.info/v1/query")
	params := url.Values{}
	params.Set("q", query)
	params.Set("fields", "clinvar,dbsnp,dbnsfp,cadd,gnomad_exome")
	if assembly != "" {
		params.Set("assembly", assembly)
	}
	u.RawQuery = params.Encode()

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("external source error: %s", strings.TrimSpace(string(body)))
	}
	return io.ReadAll(resp.Body)
}

func (a *App) externalVariantsHandler(w http.ResponseWriter, r *http.Request) {
	if a.db == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "database not configured"})
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}

	q := r.URL.Query()
	variantID := q.Get("variant_id")
	rsid := q.Get("rs_id")
	if variantID == "" && rsid == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "variant_id or rs_id is required"})
		return
	}

	var rows *sql.Rows
	var err error
	if variantID != "" {
		id, convErr := strconv.Atoi(variantID)
		if convErr != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid variant_id"})
			return
		}
		userID := userIDFromContext(r.Context())
		if userID == 0 || !a.variantOwnedByUser(id, userID) {
			writeJSON(w, http.StatusForbidden, errorResponse{Error: "forbidden"})
			return
		}
		rows, err = a.db.Query(`
			SELECT external_id, variant_id, rs_id, source, payload, fetched_at
			FROM external_variant_data
			WHERE variant_id = $1
			ORDER BY fetched_at DESC`, id)
	} else {
		rows, err = a.db.Query(`
			SELECT external_id, variant_id, rs_id, source, payload, fetched_at
			FROM external_variant_data
			WHERE rs_id = $1
			ORDER BY fetched_at DESC`, rsid)
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	defer rows.Close()

	type externalRow struct {
		ExternalID int64           `json:"external_id"`
		VariantID  *int            `json:"variant_id,omitempty"`
		RSID       *string         `json:"rs_id,omitempty"`
		Source     string          `json:"source"`
		Payload    json.RawMessage `json:"payload"`
		FetchedAt  time.Time       `json:"fetched_at"`
	}
	var out []externalRow
	for rows.Next() {
		var row externalRow
		var payload []byte
		if err := rows.Scan(&row.ExternalID, &row.VariantID, &row.RSID, &row.Source, &payload, &row.FetchedAt); err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
			return
		}
		row.Payload = json.RawMessage(payload)
		out = append(out, row)
	}
	writeJSON(w, http.StatusOK, out)
}

// Uploads

func (a *App) uploadVCFHandler(w http.ResponseWriter, r *http.Request) {
	if a.db == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "database not configured"})
		return
	}
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}

	patientID := intQuery(r.URL.Query(), "patient_id")
	sampleID := intQuery(r.URL.Query(), "sample_id")
	genomeBuild := r.URL.Query().Get("genome_build")
	if genomeBuild == "" {
		genomeBuild = "GRCh38"
	}
	userID := userIDFromContext(r.Context())
	if patientID > 0 && !a.patientOwnedByUser(patientID, userID) {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "forbidden"})
		return
	}

	path, err := a.saveUpload(r, "file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	count, err := a.importVCF(path, patientID, sampleID, genomeBuild)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "variants_imported": count})
}

func (a *App) uploadCSVHandler(w http.ResponseWriter, r *http.Request) {
	if a.db == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "database not configured"})
		return
	}
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}

	patientID := intQuery(r.URL.Query(), "patient_id")
	sampleID := intQuery(r.URL.Query(), "sample_id")
	genomeBuild := r.URL.Query().Get("genome_build")
	if genomeBuild == "" {
		genomeBuild = "GRCh38"
	}
	userID := userIDFromContext(r.Context())
	if patientID > 0 && !a.patientOwnedByUser(patientID, userID) {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "forbidden"})
		return
	}

	path, err := a.saveUpload(r, "file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	count, err := a.importCSV(path, patientID, sampleID, genomeBuild)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "variants_imported": count})
}

func (a *App) uploadFASTQHandler(w http.ResponseWriter, r *http.Request) {
	if a.db == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "database not configured"})
		return
	}
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}

	patientID := intQueryRequired(w, r.URL.Query(), "patient_id")
	if patientID == 0 {
		return
	}
	userID := userIDFromContext(r.Context())
	if !a.patientOwnedByUser(patientID, userID) {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "forbidden"})
		return
	}
	sampleName := r.URL.Query().Get("sample_name")
	if sampleName == "" {
		sampleName = fmt.Sprintf("sample-%d", time.Now().Unix())
	}
	genomeBuild := r.URL.Query().Get("genome_build")
	if genomeBuild == "" {
		genomeBuild = "GRCh38"
	}

	read1, err := a.saveUpload(r, "read1")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	read2, err := a.saveUploadOptional(r, "read2")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	var sampleID int
	err = a.db.QueryRow(`
		INSERT INTO sample (patient_id, sample_name, sample_type, sequencing_type, processing_status)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING sample_id`,
		patientID, sampleName, "genomic", "WGS", "uploaded").Scan(&sampleID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	var readID int64
	err = a.db.QueryRow(`
		INSERT INTO sample_read (sample_id, read1_path, read2_path)
		VALUES ($1,$2,$3)
		RETURNING read_id`,
		sampleID, read1, read2).Scan(&readID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	var jobID int64
	err = a.db.QueryRow(`
		INSERT INTO variant_call_job (patient_id, sample_id, read_id, genome_build)
		VALUES ($1,$2,$3,$4)
		RETURNING job_id`,
		patientID, sampleID, readID, genomeBuild).Scan(&jobID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "ok",
		"sample_id": sampleID,
		"read_id":   readID,
		"job_id":    jobID,
	})
}

// Jobs

func (a *App) jobsHandler(w http.ResponseWriter, r *http.Request) {
	if a.db == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "database not configured"})
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	userID := userIDFromContext(r.Context())
	if userID == 0 {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "unauthorized"})
		return
	}
	rows, err := a.db.Query(`
		SELECT job_id, patient_id, sample_id, read_id, genome_build, status, vcf_path, log_path, error_text,
			created_at, started_at, finished_at
		FROM variant_call_job
		WHERE patient_id IN (SELECT patient_id FROM patient WHERE owner_user_id = $1)
		ORDER BY job_id DESC LIMIT 200`, userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	defer rows.Close()

	type job struct {
		JobID       int64      `json:"job_id"`
		PatientID   *int       `json:"patient_id,omitempty"`
		SampleID    *int       `json:"sample_id,omitempty"`
		ReadID      *int64     `json:"read_id,omitempty"`
		GenomeBuild *string    `json:"genome_build,omitempty"`
		Status      string     `json:"status"`
		VCFPath     *string    `json:"vcf_path,omitempty"`
		LogPath     *string    `json:"log_path,omitempty"`
		ErrorText   *string    `json:"error_text,omitempty"`
		CreatedAt   time.Time  `json:"created_at"`
		StartedAt   *time.Time `json:"started_at,omitempty"`
		FinishedAt  *time.Time `json:"finished_at,omitempty"`
	}
	var out []job
	for rows.Next() {
		var j job
		if err := rows.Scan(&j.JobID, &j.PatientID, &j.SampleID, &j.ReadID, &j.GenomeBuild, &j.Status, &j.VCFPath, &j.LogPath, &j.ErrorText, &j.CreatedAt, &j.StartedAt, &j.FinishedAt); err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
			return
		}
		out = append(out, j)
	}
	writeJSON(w, http.StatusOK, out)
}

// Clinical summary

func (a *App) variantClinicalHandler(w http.ResponseWriter, r *http.Request) {
	if a.db == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "database not configured"})
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	variantID := r.URL.Query().Get("variant_id")
	if variantID == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "variant_id is required"})
		return
	}
	id, err := strconv.Atoi(variantID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid variant_id"})
		return
	}
	userID := userIDFromContext(r.Context())
	if userID == 0 || !a.variantOwnedByUser(id, userID) {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "forbidden"})
		return
	}

	var clinvarSig *string
	var clinvarID *string
	_ = a.db.QueryRow(`
		SELECT clinvar_significance, clinvar_id
		FROM variant_annotation
		WHERE variant_id = $1`, id).Scan(&clinvarSig, &clinvarID)

	rows, err := a.db.Query(`
		SELECT d.disease_name
		FROM variant_disease vd
		JOIN disease d ON d.disease_id = vd.disease_id
		WHERE vd.variant_id = $1
		ORDER BY d.disease_name`, id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	defer rows.Close()
	var diseases []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
			return
		}
		diseases = append(diseases, name)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"variant_id":           id,
		"clinvar_significance": clinvarSig,
		"clinvar_id":           clinvarID,
		"diseases":             diseases,
	})
}

// Exports

func (a *App) exportPatientsHandler(w http.ResponseWriter, r *http.Request) {
	if a.db == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "database not configured"})
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	userID := userIDFromContext(r.Context())
	format := exportFormat(r.URL.Query())
	rows, err := a.db.Query(`
		SELECT patient_id, external_id, first_name, last_name, date_of_birth, sex, phone_number, email, address, phenotype_description, created_at, updated_at
		FROM patient WHERE owner_user_id = $1 ORDER BY patient_id`, userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	defer rows.Close()

	if format == "csv" {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=\"patients.csv\"")
		cw := csv.NewWriter(w)
		_ = cw.Write([]string{"patient_id", "external_id", "first_name", "last_name", "date_of_birth", "sex", "phone_number", "email", "address", "phenotype_description", "created_at", "updated_at"})
		for rows.Next() {
			var p patientOutput
			if err := rows.Scan(&p.PatientID, &p.ExternalID, &p.FirstName, &p.LastName, &p.DateOfBirth, &p.Sex, &p.PhoneNumber, &p.Email, &p.Address, &p.PhenotypeDescription, &p.CreatedAt, &p.UpdatedAt); err != nil {
				return
			}
			_ = cw.Write([]string{
				strconv.Itoa(p.PatientID),
				p.ExternalID,
				p.FirstName,
				p.LastName,
				p.DateOfBirth.Format("2006-01-02"),
				p.Sex,
				deref(p.PhoneNumber),
				deref(p.Email),
				deref(p.Address),
				deref(p.PhenotypeDescription),
				p.CreatedAt.Format(time.RFC3339),
				p.UpdatedAt.Format(time.RFC3339),
			})
		}
		cw.Flush()
		return
	}

	var out []patientOutput
	for rows.Next() {
		var p patientOutput
		if err := rows.Scan(&p.PatientID, &p.ExternalID, &p.FirstName, &p.LastName, &p.DateOfBirth, &p.Sex, &p.PhoneNumber, &p.Email, &p.Address, &p.PhenotypeDescription, &p.CreatedAt, &p.UpdatedAt); err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
			return
		}
		out = append(out, p)
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *App) exportVariantsHandler(w http.ResponseWriter, r *http.Request) {
	if a.db == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "database not configured"})
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	format := exportFormat(r.URL.Query())
	rows, err := a.db.Query(`
		SELECT v.variant_id, v.chromosome, v.position, v.reference_allele, v.alternate_allele, v.rs_id, v.genome_build, v.variant_type, v.created_at
		FROM variant v
		ORDER BY v.variant_id`)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	defer rows.Close()

	if format == "csv" {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=\"variants.csv\"")
		cw := csv.NewWriter(w)
		_ = cw.Write([]string{"variant_id", "chromosome", "position", "reference_allele", "alternate_allele", "rs_id", "genome_build", "variant_type", "created_at"})
		for rows.Next() {
			var v variantOutput
			if err := rows.Scan(&v.VariantID, &v.Chromosome, &v.Position, &v.ReferenceAllele, &v.AlternateAllele, &v.RSID, &v.GenomeBuild, &v.VariantType, &v.CreatedAt); err != nil {
				return
			}
			created := ""
			if v.CreatedAt != nil {
				created = v.CreatedAt.Format(time.RFC3339)
			}
			_ = cw.Write([]string{
				strconv.Itoa(v.VariantID),
				v.Chromosome,
				strconv.FormatInt(v.Position, 10),
				v.ReferenceAllele,
				v.AlternateAllele,
				deref(v.RSID),
				v.GenomeBuild,
				v.VariantType,
				created,
			})
		}
		cw.Flush()
		return
	}

	var out []variantOutput
	for rows.Next() {
		var v variantOutput
		if err := rows.Scan(&v.VariantID, &v.Chromosome, &v.Position, &v.ReferenceAllele, &v.AlternateAllele, &v.RSID, &v.GenomeBuild, &v.VariantType, &v.CreatedAt); err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
			return
		}
		out = append(out, v)
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *App) exportPatientVariantsHandler(w http.ResponseWriter, r *http.Request) {
	if a.db == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "database not configured"})
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	userID := userIDFromContext(r.Context())
	format := exportFormat(r.URL.Query())
	rows, err := a.db.Query(`
		SELECT pv.patient_variant_id, pv.patient_id, pv.variant_id, pv.sample_id, pv.zygosity, pv.quality, pv.read_depth,
			pv.allele_depth_ref, pv.allele_depth_alt, pv.genotype_quality, pv.filter_status, pv.detected_at, pv.created_at
		FROM patient_variant pv
		JOIN patient p ON p.patient_id = pv.patient_id
		WHERE p.owner_user_id = $1
		ORDER BY pv.patient_variant_id`, userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	defer rows.Close()

	type row struct {
		PatientVariantID int64
		PatientID        int
		VariantID        int
		SampleID         *int
		Zygosity         *string
		Quality          *float64
		ReadDepth        *int
		AlleleDepthRef   *int
		AlleleDepthAlt   *int
		GenotypeQuality  *int
		FilterStatus     *string
		DetectedAt       *time.Time
		CreatedAt        time.Time
	}

	if format == "csv" {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=\"patient_variants.csv\"")
		cw := csv.NewWriter(w)
		_ = cw.Write([]string{"patient_variant_id", "patient_id", "variant_id", "sample_id", "zygosity", "quality", "read_depth", "allele_depth_ref", "allele_depth_alt", "genotype_quality", "filter_status", "detected_at", "created_at"})
		for rows.Next() {
			var r row
			if err := rows.Scan(&r.PatientVariantID, &r.PatientID, &r.VariantID, &r.SampleID, &r.Zygosity, &r.Quality, &r.ReadDepth, &r.AlleleDepthRef, &r.AlleleDepthAlt, &r.GenotypeQuality, &r.FilterStatus, &r.DetectedAt, &r.CreatedAt); err != nil {
				return
			}
			detected := ""
			if r.DetectedAt != nil {
				detected = r.DetectedAt.Format(time.RFC3339)
			}
			_ = cw.Write([]string{
				strconv.FormatInt(r.PatientVariantID, 10),
				strconv.Itoa(r.PatientID),
				strconv.Itoa(r.VariantID),
				intPtrToString(r.SampleID),
				deref(r.Zygosity),
				floatPtrToString(r.Quality),
				intPtrToString(r.ReadDepth),
				intPtrToString(r.AlleleDepthRef),
				intPtrToString(r.AlleleDepthAlt),
				intPtrToString(r.GenotypeQuality),
				deref(r.FilterStatus),
				detected,
				r.CreatedAt.Format(time.RFC3339),
			})
		}
		cw.Flush()
		return
	}

	var out []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.PatientVariantID, &r.PatientID, &r.VariantID, &r.SampleID, &r.Zygosity, &r.Quality, &r.ReadDepth, &r.AlleleDepthRef, &r.AlleleDepthAlt, &r.GenotypeQuality, &r.FilterStatus, &r.DetectedAt, &r.CreatedAt); err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
			return
		}
		out = append(out, r)
	}
	writeJSON(w, http.StatusOK, out)
}

// Helpers

func readJSON(r io.Reader, v any) error {
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func methodNotAllowed(w http.ResponseWriter, allowed ...string) {
	if len(allowed) > 0 {
		w.Header().Set("Allow", strings.Join(allowed, ", "))
	}
	writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
}

func parseLimitOffset(q url.Values) (int, int) {
	limit := 100
	offset := 0
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 1000 {
			limit = n
		}
	}
	if v := q.Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	return limit, offset
}

func exportFormat(q url.Values) string {
	switch strings.ToLower(q.Get("format")) {
	case "csv":
		return "csv"
	default:
		return "json"
	}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func intPtrToString(v *int) string {
	if v == nil {
		return ""
	}
	return strconv.Itoa(*v)
}

func floatPtrToString(v *float64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatFloat(*v, 'f', -1, 64)
}

func isSexValid(v string) bool {
	switch strings.ToLower(v) {
	case "male", "female", "other":
		return true
	default:
		return false
	}
}

func isGenomeBuildValid(v string) bool {
	return v == "GRCh37" || v == "GRCh38"
}

func isVariantTypeValid(v string) bool {
	switch v {
	case "SNV", "INS", "DEL", "INDEL", "MNV", "CNV", "SV", "OTHER":
		return true
	default:
		return false
	}
}

func isZygosityValid(v string) bool {
	switch v {
	case "heterozygous", "homozygous", "hemizygous":
		return true
	default:
		return false
	}
}

func isImpactValid(v string) bool {
	switch v {
	case "HIGH", "MODERATE", "LOW", "MODIFIER":
		return true
	default:
		return false
	}
}

func isSiftValid(v string) bool {
	return v == "tolerated" || v == "deleterious"
}

func isPolyphenValid(v string) bool {
	switch v {
	case "benign", "possibly_damaging", "probably_damaging":
		return true
	default:
		return false
	}
}

func isClinvarValid(v string) bool {
	switch v {
	case "benign", "likely_benign", "uncertain_significance", "likely_pathogenic", "pathogenic", "conflicting", "not_provided":
		return true
	default:
		return false
	}
}

func isAlleleValid(v string) bool {
	if v == "-" {
		return true
	}
	for _, ch := range v {
		switch ch {
		case 'A', 'C', 'G', 'T', 'N':
			continue
		default:
			return false
		}
	}
	return true
}

func isEmailValid(v string) bool {
	if len(v) < 3 || len(v) > 254 {
		return false
	}
	if !strings.Contains(v, "@") {
		return false
	}
	parts := strings.Split(v, "@")
	if len(parts) != 2 {
		return false
	}
	return parts[0] != "" && parts[1] != ""
}

func (a *App) patientOwnedByUser(patientID, userID int) bool {
	if userID == 0 || patientID == 0 {
		return false
	}
	var exists bool
	err := a.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM patient WHERE patient_id=$1 AND owner_user_id=$2)`, patientID, userID).Scan(&exists)
	return err == nil && exists
}

func (a *App) variantOwnedByUser(variantID, userID int) bool {
	if variantID == 0 || userID == 0 {
		return false
	}
	var exists bool
	err := a.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM variant WHERE variant_id = $1)`, variantID).Scan(&exists)
	return err == nil && exists
}

func (a *App) listPatientVariants(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	patientID := q.Get("patient_id")
	if patientID == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "patient_id is required"})
		return
	}
	id, err := strconv.Atoi(patientID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid patient_id"})
		return
	}
	userID := userIDFromContext(r.Context())
	if userID == 0 || !a.patientOwnedByUser(id, userID) {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "forbidden"})
		return
	}

	rows, err := a.db.Query(`
		SELECT pv.patient_variant_id,
			   v.variant_id, v.chromosome, v.position, v.reference_allele, v.alternate_allele, v.rs_id,
			   va.clinvar_significance, g.gene_symbol,
			   COALESCE(json_agg(DISTINCT d.disease_name) FILTER (WHERE d.disease_id IS NOT NULL), '[]') AS diseases
		FROM patient_variant pv
		JOIN variant v ON v.variant_id = pv.variant_id
		LEFT JOIN variant_annotation va ON va.variant_id = v.variant_id
		LEFT JOIN gene g ON g.gene_id = va.gene_id
		LEFT JOIN variant_disease vd ON vd.variant_id = v.variant_id
		LEFT JOIN disease d ON d.disease_id = vd.disease_id
		WHERE pv.patient_id = $1
		GROUP BY pv.patient_variant_id, v.variant_id, va.clinvar_significance, g.gene_symbol
		ORDER BY pv.patient_variant_id DESC`, id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	defer rows.Close()

	type row struct {
		PatientVariantID int64    `json:"patient_variant_id"`
		VariantID        int      `json:"variant_id"`
		Chromosome       string   `json:"chromosome"`
		Position         int64    `json:"position"`
		ReferenceAllele  string   `json:"reference_allele"`
		AlternateAllele  string   `json:"alternate_allele"`
		RSID             *string  `json:"rs_id,omitempty"`
		ClinvarSign      *string  `json:"clinvar_significance,omitempty"`
		GeneSymbol       *string  `json:"gene_symbol,omitempty"`
		Diseases         []string `json:"diseases"`
	}
	var out []row
	for rows.Next() {
		var r row
		var diseasesRaw []byte
		if err := rows.Scan(&r.PatientVariantID, &r.VariantID, &r.Chromosome, &r.Position, &r.ReferenceAllele, &r.AlternateAllele, &r.RSID, &r.ClinvarSign, &r.GeneSymbol, &diseasesRaw); err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
			return
		}
		_ = json.Unmarshal(diseasesRaw, &r.Diseases)
		out = append(out, r)
	}
	writeJSON(w, http.StatusOK, out)
}

// Import helpers

func (a *App) importVCF(path string, patientID, sampleID int, genomeBuild string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	reader := bufio.NewReader(f)
	count := 0
	for {
		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return count, err
		}
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			fields := strings.Split(line, "\t")
			if len(fields) >= 5 {
				chr := fields[0]
				posStr := fields[1]
				id := fields[2]
				ref := fields[3]
				altField := fields[4]
				pos, convErr := strconv.ParseInt(posStr, 10, 64)
				if convErr == nil {
					alts := strings.Split(altField, ",")
					for _, alt := range alts {
						alt = strings.TrimSpace(alt)
						if alt == "" {
							continue
						}
						rsid := (*string)(nil)
						if strings.HasPrefix(id, "rs") {
							rsid = &id
						}
						variantType := guessVariantType(ref, alt)
						var variantID int
						err = a.db.QueryRow(`
							INSERT INTO variant (chromosome, position, reference_allele, alternate_allele, rs_id, genome_build, variant_type)
							VALUES ($1,$2,$3,$4,$5,$6,$7)
							ON CONFLICT (chromosome, position, reference_allele, alternate_allele, genome_build)
							DO UPDATE SET rs_id = COALESCE(variant.rs_id, EXCLUDED.rs_id)
							RETURNING variant_id`,
							chr, pos, ref, alt, rsid, genomeBuild, variantType).Scan(&variantID)
						if err != nil {
							return count, err
						}
						if patientID > 0 {
							_, _ = a.db.Exec(`
								INSERT INTO patient_variant (patient_id, variant_id, sample_id)
								VALUES ($1,$2,$3)
								ON CONFLICT (patient_id, variant_id) DO NOTHING`,
								patientID, variantID, nullableInt(sampleID))
						}
						count++
					}
				}
			}
		}
		if errors.Is(err, io.EOF) {
			break
		}
	}
	return count, nil
}

func (a *App) importCSV(path string, patientID, sampleID int, genomeBuild string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.TrimLeadingSpace = true
	header, err := reader.Read()
	if err != nil {
		return 0, err
	}
	idx := map[string]int{}
	for i, h := range header {
		idx[strings.ToLower(strings.TrimSpace(h))] = i
	}

	get := func(row []string, key string) string {
		i, ok := idx[key]
		if !ok || i >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[i])
	}

	count := 0
	for {
		row, err := reader.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return count, err
		}
		chr := get(row, "chromosome")
		posStr := get(row, "position")
		ref := get(row, "reference_allele")
		alt := get(row, "alternate_allele")
		if chr == "" || posStr == "" || ref == "" || alt == "" {
			continue
		}
		pos, convErr := strconv.ParseInt(posStr, 10, 64)
		if convErr != nil {
			continue
		}
		rsidStr := get(row, "rs_id")
		var rsid *string
		if rsidStr != "" {
			rsid = &rsidStr
		}
		gb := get(row, "genome_build")
		if gb == "" {
			gb = genomeBuild
		}
		vt := get(row, "variant_type")
		if vt == "" {
			vt = guessVariantType(ref, alt)
		}
		var variantID int
		err = a.db.QueryRow(`
			INSERT INTO variant (chromosome, position, reference_allele, alternate_allele, rs_id, genome_build, variant_type)
			VALUES ($1,$2,$3,$4,$5,$6,$7)
			ON CONFLICT (chromosome, position, reference_allele, alternate_allele, genome_build)
			DO UPDATE SET rs_id = COALESCE(variant.rs_id, EXCLUDED.rs_id)
			RETURNING variant_id`,
			chr, pos, ref, alt, rsid, gb, vt).Scan(&variantID)
		if err != nil {
			return count, err
		}
		if patientID > 0 {
			_, _ = a.db.Exec(`
				INSERT INTO patient_variant (patient_id, variant_id, sample_id)
				VALUES ($1,$2,$3)
				ON CONFLICT (patient_id, variant_id) DO NOTHING`,
				patientID, variantID, nullableInt(sampleID))
		}
		count++
	}
	return count, nil
}

func (a *App) saveUpload(r *http.Request, field string) (string, error) {
	if err := r.ParseMultipartForm(512 << 20); err != nil {
		return "", err
	}
	file, header, err := r.FormFile(field)
	if err != nil {
		return "", err
	}
	defer file.Close()

	if err := os.MkdirAll(a.storageDir, 0o755); err != nil {
		return "", err
	}
	name := fmt.Sprintf("%d_%s", time.Now().UnixNano(), filepath.Base(header.Filename))
	path := filepath.Join(a.storageDir, name)
	out, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer out.Close()
	if _, err := io.Copy(out, file); err != nil {
		return "", err
	}
	return path, nil
}

func (a *App) saveUploadOptional(r *http.Request, field string) (*string, error) {
	file, header, err := r.FormFile(field)
	if err != nil {
		return nil, nil
	}
	defer file.Close()
	if err := os.MkdirAll(a.storageDir, 0o755); err != nil {
		return nil, err
	}
	name := fmt.Sprintf("%d_%s", time.Now().UnixNano(), filepath.Base(header.Filename))
	path := filepath.Join(a.storageDir, name)
	out, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	defer out.Close()
	if _, err := io.Copy(out, file); err != nil {
		return nil, err
	}
	return &path, nil
}

func (a *App) tryAttachDisease(variantID *int, payload []byte) {
	if variantID == nil {
		return
	}
	name, sig := extractClinvarDisease(payload)
	if name == "" {
		return
	}
	if !isPathogenic(sig) {
		return
	}
	var diseaseID int
	err := a.db.QueryRow(`
		INSERT INTO disease (disease_name)
		VALUES ($1)
		ON CONFLICT (disease_name) DO UPDATE SET disease_name = EXCLUDED.disease_name
		RETURNING disease_id`, name).Scan(&diseaseID)
	if err != nil {
		return
	}
	_, _ = a.db.Exec(`
		INSERT INTO variant_disease (variant_id, disease_id, source)
		VALUES ($1,$2,$3)
		ON CONFLICT (variant_id, disease_id, source) DO NOTHING`,
		*variantID, diseaseID, "clinvar")
}

func extractClinvarDisease(payload []byte) (string, string) {
	var root map[string]any
	if err := json.Unmarshal(payload, &root); err != nil {
		return "", ""
	}
	hits, ok := root["hits"].([]any)
	if !ok || len(hits) == 0 {
		return "", ""
	}
	first, ok := hits[0].(map[string]any)
	if !ok {
		return "", ""
	}
	clinvar, ok := first["clinvar"].(map[string]any)
	if !ok {
		return "", ""
	}
	significance := ""
	if cs, ok := clinvar["clinical_significance"].(map[string]any); ok {
		if desc, ok := cs["description"].(string); ok {
			significance = desc
		}
	}
	if rcv, ok := clinvar["rcv"].([]any); ok && len(rcv) > 0 {
		if rcv0, ok := rcv[0].(map[string]any); ok {
			if cond, ok := rcv0["conditions"].(map[string]any); ok {
				if name, ok := cond["name"].(string); ok {
					return name, significance
				}
			}
		}
	}
	return "", significance
}

func isPathogenic(sig string) bool {
	s := strings.ToLower(sig)
	return strings.Contains(s, "pathogenic")
}

func intQuery(values url.Values, key string) int {
	if v := values.Get(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return 0
}

func intQueryRequired(w http.ResponseWriter, values url.Values, key string) int {
	n := intQuery(values, key)
	if n == 0 {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: fmt.Sprintf("%s is required", key)})
		return 0
	}
	return n
}

func nullableInt(v int) any {
	if v == 0 {
		return nil
	}
	return v
}

func guessVariantType(ref, alt string) string {
	if len(ref) == 1 && len(alt) == 1 {
		return "SNV"
	}
	if len(ref) < len(alt) {
		return "INS"
	}
	if len(ref) > len(alt) {
		return "DEL"
	}
	return "INDEL"
}

// Pipeline (FASTQ -> VCF)

func (a *App) StartPipelineWorker() {
	if a.db == nil {
		return
	}
	if strings.ToLower(envOrDefault("DGV_PIPELINE_ENABLED", "false")) != "true" {
		return
	}
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			a.runNextJob()
		}
	}()
}

func (a *App) runNextJob() {
	var jobID int64
	var readID int64
	var patientID *int
	var sampleID *int
	var genomeBuild *string
	err := a.db.QueryRow(`
		SELECT job_id, read_id, patient_id, sample_id, genome_build
		FROM variant_call_job
		WHERE status = 'pending'
		ORDER BY job_id
		LIMIT 1`).Scan(&jobID, &readID, &patientID, &sampleID, &genomeBuild)
	if err != nil {
		return
	}
	_, _ = a.db.Exec(`UPDATE variant_call_job SET status='running', started_at=NOW() WHERE job_id=$1`, jobID)

	var read1 string
	var read2 *string
	err = a.db.QueryRow(`SELECT read1_path, read2_path FROM sample_read WHERE read_id = $1`, readID).Scan(&read1, &read2)
	if err != nil {
		a.failJob(jobID, err)
		return
	}

	ref := envOrDefault("DGV_REFERENCE_FASTA", "")
	if ref == "" {
		a.failJob(jobID, fmt.Errorf("DGV_REFERENCE_FASTA not set"))
		return
	}

	jobDir := filepath.Join(a.storageDir, fmt.Sprintf("job_%d", jobID))
	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		a.failJob(jobID, err)
		return
	}
	samPath := filepath.Join(jobDir, "aligned.sam")
	bamPath := filepath.Join(jobDir, "aligned.bam")
	vcfPath := filepath.Join(jobDir, "variants.vcf.gz")
	logPath := filepath.Join(jobDir, "pipeline.log")

	if err := runPipeline(ref, read1, read2, samPath, bamPath, vcfPath, logPath); err != nil {
		a.failJob(jobID, err)
		return
	}

	pid := 0
	if patientID != nil {
		pid = *patientID
	}
	sid := 0
	if sampleID != nil {
		sid = *sampleID
	}
	gb := "GRCh38"
	if genomeBuild != nil && *genomeBuild != "" {
		gb = *genomeBuild
	}
	if _, err := a.importVCF(vcfPath, pid, sid, gb); err != nil {
		a.failJob(jobID, err)
		return
	}
	_, _ = a.db.Exec(`UPDATE variant_call_job SET status='done', vcf_path=$1, log_path=$2, finished_at=NOW() WHERE job_id=$3`, vcfPath, logPath, jobID)
}

func (a *App) failJob(jobID int64, err error) {
	_, _ = a.db.Exec(`UPDATE variant_call_job SET status='failed', error_text=$1, finished_at=NOW() WHERE job_id=$2`, err.Error(), jobID)
}

func runPipeline(ref, read1 string, read2 *string, samPath, bamPath, vcfPath, logPath string) error {
	logf, err := os.Create(logPath)
	if err != nil {
		return err
	}
	defer logf.Close()

	bwa := envOrDefault("DGV_TOOL_BWA", "bwa")
	samtools := envOrDefault("DGV_TOOL_SAMTOOLS", "samtools")
	bcftools := envOrDefault("DGV_TOOL_BCFTOOLS", "bcftools")

	args := []string{"mem", ref, read1}
	if read2 != nil && *read2 != "" {
		args = append(args, *read2)
	}
	cmdBwa := exec.Command(bwa, args...)
	cmdBwa.Stdout = mustCreate(samPath)
	cmdBwa.Stderr = logf
	if err := cmdBwa.Run(); err != nil {
		return err
	}

	cmdSort := exec.Command(samtools, "sort", "-o", bamPath, samPath)
	cmdSort.Stdout = logf
	cmdSort.Stderr = logf
	if err := cmdSort.Run(); err != nil {
		return err
	}

	cmdIndex := exec.Command(samtools, "index", bamPath)
	cmdIndex.Stdout = logf
	cmdIndex.Stderr = logf
	if err := cmdIndex.Run(); err != nil {
		return err
	}

	cmdMp := exec.Command(bcftools, "mpileup", "-f", ref, "-Ou", bamPath)
	cmdCall := exec.Command(bcftools, "call", "-mv", "-Oz", "-o", vcfPath)
	pipe, err := cmdMp.StdoutPipe()
	if err != nil {
		return err
	}
	cmdMp.Stderr = logf
	cmdCall.Stdin = pipe
	cmdCall.Stdout = logf
	cmdCall.Stderr = logf
	if err := cmdMp.Start(); err != nil {
		return err
	}
	if err := cmdCall.Start(); err != nil {
		return err
	}
	if err := cmdMp.Wait(); err != nil {
		return err
	}
	if err := cmdCall.Wait(); err != nil {
		return err
	}

	cmdIndexVcf := exec.Command(bcftools, "index", vcfPath)
	cmdIndexVcf.Stdout = logf
	cmdIndexVcf.Stderr = logf
	if err := cmdIndexVcf.Run(); err != nil {
		return err
	}

	return nil
}

func mustCreate(path string) *os.File {
	f, err := os.Create(path)
	if err != nil {
		return os.Stdout
	}
	return f
}
