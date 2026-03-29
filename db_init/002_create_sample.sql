DO $$
BEGIN
    CREATE TYPE seq_type AS ENUM('WGS', 'WES', 'PANEL', 'RNA-SEQ', 'OTHER');
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

DO $$
BEGIN
    CREATE TYPE status AS ENUM('uploaded','processing','annotated','completed','failed');
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS sample (
    sample_id SERIAL PRIMARY KEY,
    patient_id INT NOT NULL,
    sample_name VARCHAR(100) NOT NULL,
    sample_type VARCHAR(50) NOT NULL,
    sequencing_type seq_type NOT NULL,
    panel_name VARCHAR(100),
    sequencing_platform VARCHAR(100),
    sequencing_date DATE,
    mean_coverage DECIMAL(8,2),
    vcf_file_path VARCHAR(500),
    processing_status status NOT NULL DEFAULT 'uploaded',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (patient_id) REFERENCES patient(patient_id)
);
