DO $$
BEGIN
    CREATE TYPE inheritance_pattern AS ENUM(
        'AD',
        'AR',
        'XLD',
        'XLR',
        'MT',
        'MULTI',
        'UNKNOWN'
    );
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS disease (
    disease_id SERIAL PRIMARY KEY,
    disease_name VARCHAR(500) NOT NULL,
    omim_phenotype_id VARCHAR(10),
    orpha_code VARCHAR(20),
    inheritance_pattern inheritance_pattern DEFAULT 'UNKNOWN',
    description TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_disease_name_unique ON disease (disease_name);

CREATE INDEX IF NOT EXISTS idx_disease_omim_phenotype ON disease (omim_phenotype_id);
CREATE INDEX IF NOT EXISTS idx_disease_orpha_code ON disease (orpha_code);
CREATE INDEX IF NOT EXISTS idx_disease_name ON disease (disease_name);
