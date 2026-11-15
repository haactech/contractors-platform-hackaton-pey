import type { Account, AuthConfig, AuthErrorCode, FieldErrors } from '../domain/auth';

export interface AuthGateway {
  config(signal?: AbortSignal): Promise<AuthConfig>;
  session(signal?: AbortSignal): Promise<Account | null>;
  signUp(input: { email: string; phone: string; pin: string }): Promise<Account>;
  logIn(input: { identifier: string; pin: string }): Promise<Account>;
  logInDemo(): Promise<Account>;
  logOut(): Promise<void>;
  sendEmailCode(): Promise<{ devCode?: string }>;
  confirmEmail(code: string): Promise<Account>;
  startPinRecovery(identifier: string): Promise<{ devCode?: string }>;
  confirmPinRecovery(input: { identifier: string; code?: string; pin: string }): Promise<Account>;
}

export class AuthError extends Error {
  constructor(
    message: string,
    readonly fields: FieldErrors = {},
    readonly status = 0,
    readonly code?: AuthErrorCode,
    readonly retryAfterSeconds?: number,
  ) {
    super(message);
    this.name = 'AuthError';
  }
}
