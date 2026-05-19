// Зеркальные TS-типы к backend/internal/models/*.go
// Все *T в Go становятся T | null | undefined (опциональные поля с omitempty).

// ──────────────── User / Auth ────────────────

export interface User {
  user_id: number;
  username: string;
  email?: string | null;
  first_name?: string | null;
  last_name?: string | null;
  role: string;
  is_active: boolean;
  created_at?: string | null;
  updated_at?: string | null;
}

export interface UserCreate {
  username: string;
  password: string;
  email?: string | null;
  first_name?: string | null;
  last_name?: string | null;
  role?: string | null;
}

export interface Credentials {
  username: string;
  password: string;
}

export interface AuthResponse {
  token: string;
  expires_at: number; // unix seconds
  user: User;
}

// ──────────────── Patient ────────────────

export type Sex = "male" | "female" | "other";

export interface Patient {
  patient_id: number;
  external_id?: string | null;
  first_name: string;
  last_name: string;
  date_of_birth?: string | null; // YYYY-MM-DD
  sex?: Sex | string | null;
  phone_number?: string | null;
  email?: string | null;
  address?: string | null;
  phenotype_description?: string | null;
  created_at?: string | null;
  updated_at?: string | null;
}

export interface PatientCreate {
  external_id?: string | null;
  first_name: string;
  last_name: string;
  date_of_birth?: string | null;
  sex?: Sex | string | null;
  phone_number?: string | null;
  email?: string | null;
  address?: string | null;
  phenotype_description?: string | null;
}

export type PatientUpdate = Partial<PatientCreate>;

// ──────────────── Variant ────────────────

export type VariantType =
  | "SNV"
  | "INS"
  | "DEL"
  | "INDEL"
  | "MNV"
  | "CNV"
  | "SV"
  | "OTHER";

export interface Variant {
  variant_id: number;
  chromosome: string;
  position: number;
  reference: string;
  alternate: string;
  rs_id?: string | null;
  genome_build?: string | null;
  variant_type?: VariantType | string | null;
}

export interface PatientVariant {
  patient_variant_id: number;
  patient_id: number;
  sample_id?: number | null;
  variant: Variant;
  zygosity?: string | null;
  quality?: number | null;
  read_depth?: number | null;
  allele_depth_ref?: number | null;
  allele_depth_alt?: number | null;
  genotype_quality?: number | null;
  filter_status?: string | null;
  detected_at?: string | null;
}

export interface VariantAnnotation {
  annotation_id: number;
  variant_id: number;
  gene_id?: number | null;
  consequence?: string | null;
  impact?: string | null;
  transcript_id?: string | null;
  hgvsc?: string | null;
  hgvsp?: string | null;
  protein_position?: number | null;
  amino_acids?: string | null;
  codons?: string | null;
  gnomad_af?: number | null;
  gnomad_af_popmax?: number | null;
  sift_score?: number | null;
  sift_prediction?: string | null;
  polyphen_score?: number | null;
  polyphen_prediction?: string | null;
  cadd_score?: number | null;
  revel_score?: number | null;
  clinvar_significance?: string | null;
  clinvar_id?: string | null;
}

export interface Gene {
  gene_id: number;
  gene_symbol?: string | null;
  gene_name?: string | null;
  ensembl_gene_id?: string | null;
  ncbi_gene_id?: number | null;
  omim_gene_id?: string | null;
  chromosome?: string | null;
  start_position?: number | null;
  end_position?: number | null;
  strand?: string | null;
  gene_description?: string | null;
}

export interface Phenotype {
  phenotype_id: number;
  phenotype_name: string;
  omim_phenotype_id?: string | null;
  orpha_code?: string | null;
  inheritance_pattern?: string | null;
  description?: string | null;
}

export interface VariantInterpretation {
  interpretation_id: number;
  patient_variant_id: number;
  user_id: number;
  acmg_classification: string;
  interpretation_text?: string | null;
  disease_id?: number | null;
  created_at?: string | null;
  updated_at?: string | null;
}

export interface VariantDetails {
  variant: Variant;
  annotations: VariantAnnotation[] | null;
  genes: Gene[] | null;
  phenotypes: Phenotype[] | null;
  interpretations: VariantInterpretation[] | null;
  patient_count: number;
}

export interface PatientVariantRich extends PatientVariant {
  annotations?: VariantAnnotation[];
  gene_symbols?: string[];
  top_consequence?: string | null;
  top_impact?: string | null;
}

// ──────────────── Sample ────────────────

export type SampleProcessingStatus =
  | "processing"
  | "completed"
  | "failed"
  | string;

export interface Sample {
  sample_id: number;
  patient_id: number;
  sample_name: string;
  sample_type: string;
  sequencing_type: string;
  panel_name?: string | null;
  sequencing_platform?: string | null;
  sequencing_date?: string | null;
  mean_coverage?: number | null;
  vcf_file_path?: string | null;
  processing_status: SampleProcessingStatus;
  failure_reason?: string | null;
  created_at?: string | null;
}

// Sample + поля пациента — для глобального списка загрузок (GET /api/samples).
export interface SampleWithPatient extends Sample {
  patient_external_id?: string | null;
  patient_first_name: string;
  patient_last_name: string;
}

// Результат глобального поиска по вариантам (GET /api/variants).
export interface VariantSearchResult {
  variant: Variant;
  gene_symbols: string[] | null;
  patient_count: number;
  top_impact?: string | null;
  clinvar_significance?: string | null;
}

export interface VariantSearchParams {
  q?: string;
  rs_id?: string;
  gene?: string;
  chrom?: string;
  pos?: number;
  ref?: string;
  alt?: string;
  build?: string;
  limit?: number;
  offset?: number;
}

// ──────────────── Списки / ответы ────────────────

export interface PageResponse<T> {
  items: T[];
  total: number;
  limit: number;
  offset: number;
}

export interface PatientVariantsResponse {
  patient_id: number;
  items: PatientVariantRich[];
  total: number;
  limit: number;
  offset: number;
}

export interface SampleUploadResponse {
  sample_id: number;
  patient_id: number;
  status: SampleProcessingStatus;
}

// ──────────────── Фильтры запросов ────────────────

export interface PatientListParams {
  q?: string;
  limit?: number;
  offset?: number;
}

export interface VariantListParams {
  chrom?: string;
  variant_type?: VariantType | string;
  filter?: string;
  min_qual?: number;
  zygosity?: string;
  q?: string;
  sort?: string;
  limit?: number;
  offset?: number;
}

export interface CohortListParams {
  limit?: number;
  offset?: number;
}

// ──────────────── Ошибка API ────────────────

export interface ApiError {
  error: string;
  request_id?: string;
}
