-- View: variants with full annotation
CREATE OR REPLACE VIEW v_variant_full AS
SELECT
    v.variant_id,
    v.chromosome,
    v.position,
    v.reference_allele,
    v.alternate_allele,
    v.rs_id,
    v.variant_type,
    v.genome_build,
    va.consequence,
    va.impact,
    va.hgvsc,
    va.hgvsp,
    g.gene_symbol,
    g.gene_name,
    va.gnomad_af,
    va.clinvar_significance,
    va.sift_prediction,
    va.polyphen_prediction,
    va.cadd_score
FROM variant v
LEFT JOIN variant_annotation va ON v.variant_id = va.variant_id
LEFT JOIN gene g ON va.gene_id = g.gene_id;

-- View: patients with their variants
CREATE OR REPLACE VIEW v_patient_variants AS
SELECT
    p.patient_id,
    p.external_id,
    v.variant_id,
    v.chromosome,
    v.position,
    v.reference_allele,
    v.alternate_allele,
    v.rs_id,
    pv.zygosity,
    pv.quality,
    va.consequence,
    va.impact,
    g.gene_symbol,
    va.clinvar_significance
FROM patient p
JOIN patient_variant pv ON p.patient_id = pv.patient_id
JOIN variant v ON pv.variant_id = v.variant_id
LEFT JOIN variant_annotation va ON v.variant_id = va.variant_id
LEFT JOIN gene g ON va.gene_id = g.gene_id;

-- View: variant statistics
CREATE OR REPLACE VIEW v_variant_statistics AS
SELECT
    v.variant_id,
    v.chromosome,
    v.position,
    v.reference_allele,
    v.alternate_allele,
    v.rs_id,
    COUNT(pv.patient_id) AS patient_count,
    string_agg(DISTINCT p.external_id, ',') AS patient_ids
FROM variant v
LEFT JOIN patient_variant pv ON v.variant_id = pv.variant_id
LEFT JOIN patient p ON pv.patient_id = p.patient_id
GROUP BY v.variant_id;
