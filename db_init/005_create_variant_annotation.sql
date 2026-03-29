DO $$
BEGIN
    CREATE TYPE impact_type AS ENUM('HIGH','MODERATE','LOW','MODIFIER');
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

DO $$
BEGIN
    CREATE TYPE sift_prediction_type AS ENUM('tolerated','deleterious');
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

DO $$
BEGIN
    CREATE TYPE polyphen_prediction_type AS ENUM('benign','possibly_damaging','probably_damaging');
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

DO $$
BEGIN
    CREATE TYPE clinvar_significance_type AS ENUM('benign','likely_benign','uncertain_significance','likely_pathogenic','pathogenic','conflicting','not_provided');
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS variant_annotation (
	annotation_id BIGSERIAL PRIMARY KEY,
	variant_id INT NOT NULL REFERENCES variant(variant_id),
	gene_id INT REFERENCES gene(gene_id),
	consequence VARCHAR(100),
	impact impact_type,
	transcript_id VARCHAR(30),
	hgvsc VARCHAR(255),
	hgvsp VARCHAR(255),
	protein_position INT,
	amino_acids VARCHAR(50),
	codons VARCHAR(50),
	gnomad_af DECIMAL(10,8),
	gnomad_af_popmax DECIMAL(10,8),
	sift_score DECIMAL(5,4),
	sift_prediction sift_prediction_type,
	polyphen_score DECIMAL(5,4),
	polyphen_prediction polyphen_prediction_type,
	cadd_score DECIMAL(6,3),
	revel_score DECIMAL(5,4),
	clinvar_significance clinvar_significance_type,
	clinvar_id VARCHAR(20),
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_variant_annotation_variant_id ON variant_annotation (variant_id);
