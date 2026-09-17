export interface LoginRequest {
  email: string;
  password: string;
}

export interface LoginData {
  token: string;
  expires_at: string;
}

export interface LoginSuccessResponse {
  success: true;
  data: LoginData;
}

export interface ApiErrorInfo {
  code: string;
  message: string;
}

export interface ApiErrorResponse {
  success: false;
  error: ApiErrorInfo;
}

export type LoginFailureKind = 'http' | 'network' | 'unexpected';

export class LoginApiError extends Error {
  readonly kind: LoginFailureKind;
  readonly status: number | null;
  readonly code: string | null;

  constructor(kind: LoginFailureKind, message: string, status: number | null, code: string | null) {
    super(message);
    this.name = 'LoginApiError';
    this.kind = kind;
    this.status = status;
    this.code = code;
  }
}
