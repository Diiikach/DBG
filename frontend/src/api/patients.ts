import { apiClient } from "./client";
import { cleanParams } from "./params";
import type {
  PageResponse,
  Patient,
  PatientCreate,
  PatientListParams,
  PatientUpdate,
} from "../types/api";

export async function listPatients(
  params: PatientListParams = {},
): Promise<PageResponse<Patient>> {
  const { data } = await apiClient.get<PageResponse<Patient>>("/api/patients", {
    params: cleanParams({ ...params }),
  });
  return data;
}

export async function getPatient(id: number): Promise<Patient> {
  const { data } = await apiClient.get<Patient>(`/api/patients/${id}`);
  return data;
}

export async function createPatient(payload: PatientCreate): Promise<Patient> {
  const { data } = await apiClient.post<Patient>("/api/patients", payload);
  return data;
}

export async function updatePatient(
  id: number,
  payload: PatientUpdate,
): Promise<Patient> {
  const { data } = await apiClient.patch<Patient>(`/api/patients/${id}`, payload);
  return data;
}

export async function deletePatient(id: number): Promise<void> {
  await apiClient.delete(`/api/patients/${id}`);
}
