DO $$
BEGIN
    CREATE TYPE zygosity_type AS ENUM('heterozygous','homozygous','hemizygous');
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS patient_variant (
	patient_variant_id BIGSERIAL PRIMARY KEY,
	patient_id INT NOT NULL REFERENCES patient(patient_id),
	variant_id INT NOT NULL REFERENCES variant(variant_id),
	sample_id INT REFERENCES sample(sample_id),
	zygosity zygosity_type,
	quality DECIMAL(10,2),
	read_depth INT,
	allele_depth_ref INT,
	allele_depth_alt INT,
	genotype_quality INT,
	filter_status VARCHAR(50),
	detected_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_patient_variant_unique
	ON patient_variant (patient_id, variant_id);
