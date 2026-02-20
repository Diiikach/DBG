-- 007_create_disease.sql
-- Создание таблицы disease (Справочник заболеваний)

CREATE TABLE IF NOT EXISTS disease (
    disease_id INT AUTO_INCREMENT PRIMARY KEY,
    disease_name VARCHAR(500) NOT NULL COMMENT 'Название заболевания',
    omim_phenotype_id VARCHAR(10) NULL COMMENT 'OMIM Phenotype MIM number',
    orpha_code VARCHAR(20) NULL COMMENT 'Orphanet код (для редких заболеваний)',
    inheritance_pattern ENUM(
        'AD',      -- Аутосомно-доминантный
        'AR',      -- Аутосомно-рецессивный
        'XLD',     -- X-linked dominant
        'XLR',     -- X-linked recessive
        'MT',      -- Митохондриальный
        'MULTI',   -- Многофакторный
        'UNKNOWN'  -- Неизвестен
    ) DEFAULT 'UNKNOWN' COMMENT 'Тип наследования',
    description TEXT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_omim_phenotype (omim_phenotype_id),
    INDEX idx_orpha_code (orpha_code),
    INDEX idx_disease_name (disease_name(100))
) COMMENT 'Справочник заболеваний';
