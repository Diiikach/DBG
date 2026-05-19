package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/term-paper-2026/backend/internal/models"
)

// ErrNotFound — запись не найдена.
var ErrNotFound = errors.New("not found")

type PatientRepo struct {
	pool *pgxpool.Pool
}

func NewPatientRepo(p *pgxpool.Pool) *PatientRepo { return &PatientRepo{pool: p} }

const patientCols = `patient_id, external_id, first_name, last_name,
		to_char(date_of_birth,'YYYY-MM-DD'), sex::text,
		phone_number, email, address, phenotype_description,
		created_at, updated_at`

// isolationCond — фрагмент WHERE, гарантирующий, что пациент доступен только
// своему создателю. Legacy-записи (created_by_user_id IS NULL) пока считаются
// общими, чтобы не сломать существующие данные до бэкфилла.
const isolationCond = `(created_by_user_id = $%d OR created_by_user_id IS NULL)`

func scanPatient(row pgx.Row, p *models.Patient) error {
	return row.Scan(
		&p.PatientID, &p.ExternalID, &p.FirstName, &p.LastName,
		&p.DateOfBirth, &p.Sex, &p.PhoneNumber, &p.Email, &p.Address,
		&p.PhenotypeDescription, &p.CreatedAt, &p.UpdatedAt,
	)
}

// Create вставляет нового пациента с привязкой к создавшему его пользователю.
func (r *PatientRepo) Create(ctx context.Context, in models.PatientCreate, userID int) (models.Patient, error) {
	const q = `
		INSERT INTO patient
			(external_id, first_name, last_name, date_of_birth, sex,
			 phone_number, email, address, phenotype_description, created_by_user_id)
		VALUES ($1, $2, $3, $4::date, $5::sex_enum, $6, $7, $8, $9, $10)
		RETURNING ` + patientCols
	args := []any{
		in.ExternalID, in.FirstName, in.LastName, in.DateOfBirth, in.Sex,
		in.PhoneNumber, in.Email, in.Address, in.PhenotypeDescription, userID,
	}
	start := time.Now()
	var p models.Patient
	err := scanPatient(r.pool.QueryRow(ctx, q, args...), &p)
	logQuery(ctx, "patient.Create", q, args, err, start)
	if err != nil {
		return models.Patient{}, fmt.Errorf("insert patient: %w", err)
	}
	return p, nil
}

// Get возвращает пациента по id, доступного указанному пользователю.
func (r *PatientRepo) Get(ctx context.Context, id, userID int) (models.Patient, error) {
	q := `SELECT ` + patientCols + ` FROM patient WHERE patient_id = $1 AND ` +
		fmt.Sprintf(isolationCond, 2)
	args := []any{id, userID}
	start := time.Now()
	var p models.Patient
	err := scanPatient(r.pool.QueryRow(ctx, q, args...), &p)
	if errors.Is(err, pgx.ErrNoRows) {
		logQuery(ctx, "patient.Get", q, args, nil, start)
		return models.Patient{}, ErrNotFound
	}
	logQuery(ctx, "patient.Get", q, args, err, start)
	if err != nil {
		return models.Patient{}, err
	}
	return p, nil
}

// List возвращает страницу пациентов текущего пользователя и общее количество.
func (r *PatientRepo) List(ctx context.Context, userID int, q string, limit, offset int) ([]models.Patient, int, error) {
	args := []any{limit, offset, userID}
	where := "WHERE " + fmt.Sprintf(isolationCond, 3)
	if q = strings.TrimSpace(q); q != "" {
		args = append(args, "%"+strings.ToLower(q)+"%")
		where += ` AND (LOWER(first_name) LIKE $4
		      OR LOWER(last_name)  LIKE $4
		      OR LOWER(COALESCE(external_id,'')) LIKE $4)`
	}
	sql := `SELECT ` + patientCols + `
		FROM patient ` + where + `
		ORDER BY patient_id DESC
		LIMIT $1 OFFSET $2`

	start := time.Now()
	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		logQuery(ctx, "patient.List", sql, args, err, start)
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]models.Patient, 0, limit)
	for rows.Next() {
		var p models.Patient
		if err := scanPatient(rows, &p); err != nil {
			logQuery(ctx, "patient.List", sql, args, err, start)
			return nil, 0, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		logQuery(ctx, "patient.List", sql, args, err, start)
		return nil, 0, err
	}
	logQuery(ctx, "patient.List", sql, args, nil, start)

	// Total
	countArgs := []any{userID}
	countSQL := `SELECT count(*) FROM patient WHERE ` + fmt.Sprintf(isolationCond, 1)
	if len(args) > 3 {
		countArgs = append(countArgs, args[3])
		countSQL += ` AND (LOWER(first_name) LIKE $2
		      OR LOWER(last_name)  LIKE $2
		      OR LOWER(COALESCE(external_id,'')) LIKE $2)`
	}
	var total int
	cstart := time.Now()
	cerr := r.pool.QueryRow(ctx, countSQL, countArgs...).Scan(&total)
	logQuery(ctx, "patient.ListCount", countSQL, countArgs, cerr, cstart)
	if cerr != nil {
		return nil, 0, cerr
	}
	return out, total, nil
}

// Update — частичное обновление полей пациента. Возвращает обновлённую запись.
func (r *PatientRepo) Update(ctx context.Context, id int, in models.PatientUpdate, userID int) (models.Patient, error) {
	sets := []string{}
	args := []any{}
	add := func(col string, val any, cast string) {
		args = append(args, val)
		ph := fmt.Sprintf("$%d", len(args))
		if cast != "" {
			ph = ph + "::" + cast
		}
		sets = append(sets, col+" = "+ph)
	}
	if in.ExternalID != nil {
		add("external_id", *in.ExternalID, "")
	}
	if in.FirstName != nil {
		add("first_name", *in.FirstName, "")
	}
	if in.LastName != nil {
		add("last_name", *in.LastName, "")
	}
	if in.DateOfBirth != nil {
		add("date_of_birth", *in.DateOfBirth, "date")
	}
	if in.Sex != nil {
		add("sex", *in.Sex, "sex_enum")
	}
	if in.PhoneNumber != nil {
		add("phone_number", *in.PhoneNumber, "")
	}
	if in.Email != nil {
		add("email", *in.Email, "")
	}
	if in.Address != nil {
		add("address", *in.Address, "")
	}
	if in.PhenotypeDescription != nil {
		add("phenotype_description", *in.PhenotypeDescription, "")
	}
	if len(sets) == 0 {
		return r.Get(ctx, id, userID)
	}
	sets = append(sets, "updated_at = CURRENT_TIMESTAMP")
	args = append(args, id, userID)
	sql := `UPDATE patient SET ` + strings.Join(sets, ", ") +
		fmt.Sprintf(` WHERE patient_id = $%d AND `+isolationCond+` RETURNING `,
			len(args)-1, len(args)) + patientCols

	start := time.Now()
	var p models.Patient
	err := scanPatient(r.pool.QueryRow(ctx, sql, args...), &p)
	if errors.Is(err, pgx.ErrNoRows) {
		logQuery(ctx, "patient.Update", sql, args, nil, start)
		return models.Patient{}, ErrNotFound
	}
	logQuery(ctx, "patient.Update", sql, args, err, start)
	if err != nil {
		return models.Patient{}, err
	}
	return p, nil
}

// Delete удаляет пациента (только своего).
func (r *PatientRepo) Delete(ctx context.Context, id, userID int) error {
	q := `DELETE FROM patient WHERE patient_id = $1 AND ` +
		fmt.Sprintf(isolationCond, 2)
	args := []any{id, userID}
	start := time.Now()
	tag, err := r.pool.Exec(ctx, q, args...)
	logQuery(ctx, "patient.Delete", q, args, err, start)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListByVariant — пациенты с указанным variant_id, доступные текущему пользователю.
// «Когорта» в публичной семантике — все носители, но конкретные пациенты раскрываются
// только врачам, создавшим их.
func (r *PatientRepo) ListByVariant(ctx context.Context, variantID, userID, limit, offset int) ([]models.Patient, int, error) {
	const q = `
		SELECT ` + patientCols + `
		FROM patient
		WHERE patient_id IN (
			SELECT DISTINCT patient_id FROM patient_variant WHERE variant_id = $3
		)
		  AND (created_by_user_id = $4 OR created_by_user_id IS NULL)
		ORDER BY patient_id DESC
		LIMIT $1 OFFSET $2
	`
	args := []any{limit, offset, variantID, userID}
	start := time.Now()
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		logQuery(ctx, "patient.ListByVariant", q, args, err, start)
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]models.Patient, 0, limit)
	for rows.Next() {
		var p models.Patient
		if err := scanPatient(rows, &p); err != nil {
			logQuery(ctx, "patient.ListByVariant", q, args, err, start)
			return nil, 0, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		logQuery(ctx, "patient.ListByVariant", q, args, err, start)
		return nil, 0, err
	}
	logQuery(ctx, "patient.ListByVariant", q, args, nil, start)

	const cq = `
		SELECT count(*) FROM patient
		WHERE patient_id IN (
			SELECT DISTINCT patient_id FROM patient_variant WHERE variant_id = $1
		)
		  AND (created_by_user_id = $2 OR created_by_user_id IS NULL)
	`
	cargs := []any{variantID, userID}
	cstart := time.Now()
	var total int
	cerr := r.pool.QueryRow(ctx, cq, cargs...).Scan(&total)
	logQuery(ctx, "patient.ListByVariantCount", cq, cargs, cerr, cstart)
	if cerr != nil {
		return nil, 0, cerr
	}
	return out, total, nil
}
