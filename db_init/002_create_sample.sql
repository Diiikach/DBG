CREATE TYPE seq_type ENUM('WGS', 'WES', 'PANEL', 'RNA-SEQ', 'OTHER');
CREATE TYPE status ENUM('uploaded','processing','annotated','completed','failed');

CREATE TABLE IF NOT EXISTS sample (
    sample_id SERIAL PRIMARY KEY,
    patient_id INT NOT NULL,
    sample_name VARCHAR(100) NOT NULL,
    sample_type VARCHAR NOT NULL,
    sequencing_type seq_type NOT NULL,
    panel_name VARCHAR(100),
    sequencing_platform varchar(100)
    sequencing_date DATE,
    mean_coverage decimal(8,2),
    vcf_file_path varchar(500),
    processing_status status NOT NULL DEFAULT 'uploaded',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (patient_id) REFERENCES patient(patient_id)
);

