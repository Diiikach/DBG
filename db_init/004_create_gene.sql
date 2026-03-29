DO $$
BEGIN
    CREATE TYPE direction AS ENUM('+','-');
EXCEPTION
    WHEN duplicate_object THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS gene (
    gene_id SERIAL PRIMARY KEY,
    gene_symbol VARCHAR(50),
    gene_name VARCHAR(255),
    ensembl_gene_id VARCHAR(20),
    ncbi_gene_id INT,
    omim_gene_id VARCHAR(10),
    chromosome VARCHAR(5),
    start_position BIGINT,
    end_position BIGINT,
    strand direction,
    gene_description TEXT
);

