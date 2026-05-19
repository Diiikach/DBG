import { createContext, useCallback, useEffect, useMemo, useState } from "react";
import type { ReactNode } from "react";
import {
  EXPIRES_STORAGE_KEY,
  TOKEN_STORAGE_KEY,
  setUnauthorizedHandler,
} from "../api/client";
import * as authApi from "../api/auth";
import type { AuthResponse, User } from "../types/api";

export interface AuthContextValue {
  user: User | null;
  token: string | null;
  isInitializing: boolean;
  isAuthenticated: boolean;
  login: (username: string, password: string) => Promise<void>;
  register: (username: string, password: string) => Promise<void>;
  logout: () => void;
}

export const AuthContext = createContext<AuthContextValue | null>(null);

function readStoredToken(): { token: string | null; expiresAt: number | null } {
  const token = localStorage.getItem(TOKEN_STORAGE_KEY);
  const expiresRaw = localStorage.getItem(EXPIRES_STORAGE_KEY);
  const expiresAt = expiresRaw ? Number(expiresRaw) : null;
  return { token, expiresAt };
}

function persistAuth(resp: AuthResponse) {
  localStorage.setItem(TOKEN_STORAGE_KEY, resp.token);
  localStorage.setItem(EXPIRES_STORAGE_KEY, String(resp.expires_at));
}

function clearAuth() {
  localStorage.removeItem(TOKEN_STORAGE_KEY);
  localStorage.removeItem(EXPIRES_STORAGE_KEY);
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [token, setToken] = useState<string | null>(null);
  const [user, setUser] = useState<User | null>(null);
  const [isInitializing, setIsInitializing] = useState(true);

  const logout = useCallback(() => {
    clearAuth();
    setToken(null);
    setUser(null);
  }, []);

  // Подвешиваем глобальный 401-handler на axios.
  useEffect(() => {
    setUnauthorizedHandler(() => {
      setToken(null);
      setUser(null);
    });
  }, []);

  // Восстановление сессии при монтировании.
  useEffect(() => {
    const { token: stored, expiresAt } = readStoredToken();
    const nowSec = Math.floor(Date.now() / 1000);
    if (!stored || (expiresAt !== null && expiresAt <= nowSec)) {
      clearAuth();
      setIsInitializing(false);
      return;
    }
    setToken(stored);
    authApi
      .me()
      .then((u) => setUser(u))
      .catch(() => {
        clearAuth();
        setToken(null);
        setUser(null);
      })
      .finally(() => setIsInitializing(false));
  }, []);

  const login = useCallback(async (username: string, password: string) => {
    const resp = await authApi.login({ username, password });
    persistAuth(resp);
    setToken(resp.token);
    setUser(resp.user);
  }, []);

  const register = useCallback(async (username: string, password: string) => {
    const resp = await authApi.register({ username, password });
    persistAuth(resp);
    setToken(resp.token);
    setUser(resp.user);
  }, []);

  const value = useMemo<AuthContextValue>(
    () => ({
      user,
      token,
      isInitializing,
      isAuthenticated: !!token,
      login,
      register,
      logout,
    }),
    [user, token, isInitializing, login, register, logout],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}
