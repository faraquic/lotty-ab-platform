import { useSyncExternalStore } from 'react';

export interface AuthSession {
  token: string;
  expiresAt: string;
}

let session: AuthSession | null = null;
const listeners = new Set<() => void>();

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

function getSnapshot(): AuthSession | null {
  return session;
}

function emit(): void {
  listeners.forEach((listener) => {
    listener();
  });
}

export function setAuthSession(token: string, expiresAt: string): void {
  session = { token, expiresAt };
  emit();
}

export function getAuthToken(): string | null {
  return session?.token ?? null;
}

export function getAuthSession(): AuthSession | null {
  return session;
}

export function clearAuthSession(): void {
  session = null;
  emit();
}

export function useAuthSession(): AuthSession | null {
  return useSyncExternalStore(subscribe, getSnapshot);
}
