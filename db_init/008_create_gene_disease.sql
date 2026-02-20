-- 008_create_gene_disease.sql
-- Создание таблицы gene_disease (Связь генов с заболеваниями)

CREATE TABLE IF NOT EXISTS gene_disease (
    gene_disease_id INT AUTO_INCREMENT PRIMARY KEY,
    gene_id INT NOT NULL,
    disease_id INT NOT NULL,
    association_type ENUM(
        'causative',
        'risk_factor',
        'modifier',
        'susceptibility',
        'protective'
    ) DEFAULT 'causative',
    evidence_level ENUM('strong', 'moderate', 'limited', 'disputed') DEFAULT 'moderate',
    source VARCHAR(50) NULL COMMENT 'Источник информации (OMIM, ClinGen, etc.)',
    source_id VARCHAR(50) NULL COMMENT 'ID записи в источнике',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (gene_id) REFERENCES gene(gene_id) ON DELETE CASCADE,
    FOREIGN KEY (disease_id) REFERENCES disease(disease_id) ON DELETE CASCADE,
    UNIQUE INDEX idx_gene_disease_unique (gene_id, disease_id),
    INDEX idx_disease_id (disease_id)
) COMMENT 'Связь генов с заболеваниями (OMIM)';
