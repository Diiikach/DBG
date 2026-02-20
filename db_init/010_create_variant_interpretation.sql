-- 010_create_variant_interpretation.sql
-- Создание таблицы variant_interpretation (Клиническая интерпретация вариантов)

CREATE TABLE IF NOT EXISTS variant_interpretation (
    interpretation_id BIGINT AUTO_INCREMENT PRIMARY KEY,
    patient_variant_id BIGINT NOT NULL COMMENT 'Конкретный вариант у пациента',
    user_id INT NOT NULL COMMENT 'Врач, сделавший интерпретацию',
    acmg_classification ENUM(
        'pathogenic',
        'likely_pathogenic',
        'uncertain_significance',
        'likely_benign',
        'benign'
    ) NOT NULL,
    acmg_criteria JSON NULL COMMENT 'Список критериев ACMG (PVS1, PS1, PM2, etc.)',
    interpretation_text TEXT NULL,
    disease_id INT NULL COMMENT 'Заболевание, с которым связан вариант',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (patient_variant_id) REFERENCES patient_variant(patient_variant_id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES user(user_id) ON DELETE RESTRICT,
    FOREIGN KEY (disease_id) REFERENCES disease(disease_id) ON DELETE SET NULL,
    INDEX idx_patient_variant_id (patient_variant_id),
    INDEX idx_acmg_classification (acmg_classification)
) COMMENT 'Клиническая интерпретация вариантов';
