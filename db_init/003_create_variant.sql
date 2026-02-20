CREATE TYPE variant_type AS ENUM('SNV','INS','DEL','INDEL','MNV','CNV','SV','OTHER');
CREATE TYPE gen_build  AS ENUM('GRCh37','GRCh38');

CREATE TABLE IF NOT EXISTS variant (
    variant_id SERIAL PRIMARY KEY,
    chromosome VARCHAR(5) NOT NULL,
    position bigint NOT NULL,
    reference_allele VARCHAR(100) NOT NULL,
    alternate_allele VARCHAR(100) NOT NULL,
    rs_id VARCHAR(50),
    genome_build gen_build NOT NULL,
    variant_type variant_type NOT NULL,
    created_at timestamp
);
