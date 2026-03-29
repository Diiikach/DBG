DO $$
BEGIN
    CREATE TYPE acmg_classification AS ENUM(
        'pathogenic',
        'likely_pathogenic',
        'uncertain_significance',
        'likely_benign',
        'benign'
    );
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS variant_interpretation (
    interpretation_id BIGSERIAL PRIMARY KEY,
    patient_variant_id BIGINT NOT NULL REFERENCES patient_variant(patient_variant_id) ON DELETE CASCADE,
    user_id INT NOT NULL REFERENCES "user"(user_id) ON DELETE RESTRICT,
    acmg_classification acmg_classification NOT NULL,
    acmg_criteria JSONB,
    interpretation_text TEXT,
    disease_id INT REFERENCES disease(disease_id) ON DELETE SET NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_variant_interpretation_patient_variant_id ON variant_interpretation (patient_variant_id);
CREATE INDEX IF NOT EXISTS idx_variant_interpretation_acmg_classification ON variant_interpretation (acmg_classification);
