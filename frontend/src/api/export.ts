import { apiClient } from "./client";
import { cleanParams } from "./params";
import type { VariantListParams } from "../types/api";

export type ExportFormat = "csv" | "json";

/**
 * Скачивает Blob c сервера и инициирует загрузку файла в браузере.
 * Имя файла берётся из заголовка Content-Disposition (бэкенд его проставляет),
 * fallback — fallbackName.
 */
async function downloadFile(
  url: string,
  params: Record<string, unknown>,
  fallbackName: string,
): Promise<void> {
  const resp = await apiClient.get<Blob>(url, {
    params: cleanParams(params),
    responseType: "blob",
  });

  const blob = resp.data;
  const disp = resp.headers["content-disposition"] as string | undefined;
  const fileName = parseFilename(disp) ?? fallbackName;

  const objectUrl = window.URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = objectUrl;
  a.download = fileName;
  document.body.appendChild(a);
  a.click();
  a.remove();
  // Чуть отложенный revoke — Safari иначе не успевает дёрнуть download.
  setTimeout(() => window.URL.revokeObjectURL(objectUrl), 1000);
}

function parseFilename(contentDisposition?: string): string | null {
  if (!contentDisposition) return null;
  const m = /filename="([^"]+)"/i.exec(contentDisposition);
  return m ? m[1] : null;
}

export async function exportPatients(
  format: ExportFormat,
  params: { q?: string } = {},
): Promise<void> {
  await downloadFile(
    "/api/patients/export",
    { ...params, format },
    `patients.${format}`,
  );
}

export async function exportPatientVariants(
  patientId: number,
  format: ExportFormat,
  params: VariantListParams = {},
): Promise<void> {
  // limit/offset для экспорта не нужны — бэкенд сам ограничивает выгрузку.
  const { limit: _limit, offset: _offset, ...rest } = params;
  void _limit;
  void _offset;
  await downloadFile(
    `/api/patients/${patientId}/variants/export`,
    { ...rest, format },
    `patient-${patientId}-variants.${format}`,
  );
}
