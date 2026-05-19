import axios from "axios";
import type { AxiosError, AxiosRequestConfig } from "axios";
import type { ApiError } from "../types/api";

export const TOKEN_STORAGE_KEY = "auth_token";
export const EXPIRES_STORAGE_KEY = "auth_expires_at";

const baseURL =
  (import.meta.env.VITE_API_BASE_URL as string | undefined) ??
  "http://localhost:8080";

export const apiClient = axios.create({
  baseURL,
  headers: { "Content-Type": "application/json" },
});

apiClient.interceptors.request.use((config) => {
  const token = localStorage.getItem(TOKEN_STORAGE_KEY);
  if (token) {
    config.headers = config.headers ?? {};
    (config.headers as Record<string, string>).Authorization = `Bearer ${token}`;
  }
  return config;
});

let onUnauthorized: (() => void) | null = null;
export function setUnauthorizedHandler(handler: () => void) {
  onUnauthorized = handler;
}

apiClient.interceptors.response.use(
  (resp) => resp,
  (error: AxiosError<ApiError>) => {
    if (error.response?.status === 401) {
      localStorage.removeItem(TOKEN_STORAGE_KEY);
      localStorage.removeItem(EXPIRES_STORAGE_KEY);
      if (onUnauthorized) onUnauthorized();
    }
    return Promise.reject(error);
  },
);

/** Достаёт человекочитаемое сообщение об ошибке из ответа бэкенда. */
export function extractErrorMessage(err: unknown, fallback = "Ошибка запроса"): string {
  if (axios.isAxiosError(err)) {
    const data = err.response?.data as ApiError | undefined;
    if (data?.error) return data.error;
    if (err.message) return err.message;
  }
  if (err instanceof Error) return err.message;
  return fallback;
}

export type RequestOpts = AxiosRequestConfig;
