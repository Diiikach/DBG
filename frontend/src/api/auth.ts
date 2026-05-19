import { apiClient } from "./client";
import type { AuthResponse, Credentials, User, UserCreate } from "../types/api";

export async function register(payload: UserCreate): Promise<AuthResponse> {
  const { data } = await apiClient.post<AuthResponse>("/api/auth/register", payload);
  return data;
}

export async function login(payload: Credentials): Promise<AuthResponse> {
  const { data } = await apiClient.post<AuthResponse>("/api/auth/login", payload);
  return data;
}

export async function me(): Promise<User> {
  const { data } = await apiClient.get<User>("/api/auth/me");
  return data;
}
