import { apiClient } from "./client";
import { cleanParams } from "./params";
import type {
  PageResponse,
  Sample,
  SampleUploadResponse,
  SampleWithPatient,
} from "../types/api";

export interface SampleListParams {
  limit?: number;
  offset?: number;
}

export async function listAllSamples(
  params: SampleListParams = {},
): Promise<PageResponse<SampleWithPatient>> {
  const { data } = await apiClient.get<PageResponse<SampleWithPatient>>(
    `/api/samples`,
    { params: cleanParams({ ...params }) },
  );
  return data;
}

export interface SampleUploadFields {
  file: File;
  sample_name?: string;
  sample_type?: string;
  sequencing_type?: string;
  panel_name?: string;
}

export async function uploadSample(
  patientId: number,
  fields: SampleUploadFields,
  onUploadProgress?: (percent: number) => void,
): Promise<SampleUploadResponse> {
  const fd = new FormData();
  fd.append("file", fields.file);
  if (fields.sample_name) fd.append("sample_name", fields.sample_name);
  if (fields.sample_type) fd.append("sample_type", fields.sample_type);
  if (fields.sequencing_type) fd.append("sequencing_type", fields.sequencing_type);
  if (fields.panel_name) fd.append("panel_name", fields.panel_name);

  const { data } = await apiClient.post<SampleUploadResponse>(
    `/api/patients/${patientId}/samples`,
    fd,
    {
      headers: { "Content-Type": "multipart/form-data" },
      onUploadProgress: (evt) => {
        if (!onUploadProgress || !evt.total) return;
        onUploadProgress(Math.round((evt.loaded / evt.total) * 100));
      },
    },
  );
  return data;
}

export async function listSamples(patientId: number): Promise<PageResponse<Sample>> {
  const { data } = await apiClient.get<PageResponse<Sample>>(
    `/api/patients/${patientId}/samples`,
  );
  return data;
}

export async function getSample(sampleId: number): Promise<Sample> {
  const { data } = await apiClient.get<Sample>(`/api/samples/${sampleId}`);
  return data;
}
