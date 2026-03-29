DO $$
BEGIN
    CREATE TYPE association_type AS ENUM(
        'causative',
        'risk_factor',
        'modifier',
        'susceptibility',
        'protective'
    );
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

DO $$
BEGIN
    CREATE TYPE evidence_level AS ENUM('strong', 'moderate', 'limited', 'disputed');
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS gene_disease (
    gene_disease_id SERIAL PRIMARY KEY,
    gene_id INT NOT NULL REFERENCES gene(gene_id) ON DELETE CASCADE,
    disease_id INT NOT NULL REFERENCES disease(disease_id) ON DELETE CASCADE,
    association_type association_type DEFAULT 'causative',
    evidence_level evidence_level DEFAULT 'moderate',
    source VARCHAR(50),
    source_id VARCHAR(50),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (gene_id, disease_id)
);

CREATE INDEX IF NOT EXISTS idx_gene_disease_disease_id ON gene_disease (disease_id);
