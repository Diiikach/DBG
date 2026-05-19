import { apiClient } from "./client";
import { cleanParams } from "./params";
import type {
  CohortListParams,
  PageResponse,
  Patient,
  PatientVariantsResponse,
  VariantDetails,
  VariantListParams,
  VariantSearchParams,
  VariantSearchResult,
} from "../types/api";

export async function searchAllVariants(
  params: VariantSearchParams = {},
): Promise<PageResponse<VariantSearchResult>> {
  const { data } = await apiClient.get<PageResponse<VariantSearchResult>>(
    `/api/variants`,
    { params: cleanParams({ ...params }) },
  );
  return data;
}

export async function listPatientVariants(
  patientId: number,
  params: VariantListParams = {},
): Promise<PatientVariantsResponse> {
  const { data } = await apiClient.get<PatientVariantsResponse>(
    `/api/patients/${patientId}/variants`,
    { params: cleanParams({ ...params }) },
  );
  return data;
}

export async function getVariantDetails(variantId: number): Promise<VariantDetails> {
  const { data } = await apiClient.get<VariantDetails>(`/api/variants/${variantId}`);
  return data;
}

export async function getVariantCohort(
  variantId: number,
  params: CohortListParams = {},
): Promise<PageResponse<Patient>> {
  const { data } = await apiClient.get<PageResponse<Patient>>(
    `/api/variants/${variantId}/patients`,
    { params: cleanParams({ ...params }) },
  );
  return data;
}
