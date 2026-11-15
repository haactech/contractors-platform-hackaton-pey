import type {
  ApiErrorResponse,
  AuthConfigDto,
  CodeIssuedResponse,
  ConfirmEmailRequest,
  ConfirmPinRecoveryRequest,
  LogInRequest,
  SessionResponse,
  SignUpRequest,
  StartPinRecoveryRequest,
} from '@platform/contracts';
import { AuthError, type AuthGateway } from '../application/auth-gateway';
import type { Account, AuthConfig, AuthErrorCode } from '../domain/auth';

function parseRetryAfter(response: Response): number | undefined {
  const raw = response.headers.get('Retry-After');
  if (!raw) return undefined;
  const parsed = Number(raw);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : undefined;
}

async function parseErrorResponse(response: Response, fallbackMessage: string): Promise<AuthError> {
  const retryAfter = parseRetryAfter(response);
  try {
    const error: ApiErrorResponse<AuthErrorCode> = await response.json();
    return new AuthError(
      error.message || fallbackMessage,
      error.errors ?? {},
      response.status,
      error.code,
      retryAfter,
    );
  } catch {
    return new AuthError(fallbackMessage, {}, response.status, undefined, retryAfter);
  }
}

async function requestJson<T>(url: string, init: RequestInit, fallbackMessage: string): Promise<T> {
  const response = await fetch(url, {
    ...init,
    headers: {
      Accept: 'application/json',
      ...(init.body ? { 'Content-Type': 'application/json' } : {}),
    },
  });
  if (!response.ok) {
    throw await parseErrorResponse(response, fallbackMessage);
  }
  return response.json() as Promise<T>;
}

export const httpAuthGateway: AuthGateway = {
  async config(signal): Promise<AuthConfig> {
    const dto = await requestJson<AuthConfigDto>(
      '/api/auth/config',
      { signal },
      'No fue posible obtener la configuración de acceso.',
    );
    return dto;
  },

  async session(signal): Promise<Account | null> {
    const response = await fetch('/api/auth/session', {
      headers: { Accept: 'application/json' },
      signal,
    });
    if (response.status === 401) return null;
    if (!response.ok) {
      throw await parseErrorResponse(response, 'No fue posible verificar tu sesión.');
    }
    const data: SessionResponse = await response.json();
    return data.account;
  },

  async signUp(input: SignUpRequest): Promise<Account> {
    const data = await requestJson<SessionResponse>(
      '/api/auth/signup',
      { method: 'POST', body: JSON.stringify(input) },
      'No fue posible crear tu cuenta. Inténtalo de nuevo.',
    );
    return data.account;
  },

  async logIn(input: LogInRequest): Promise<Account> {
    const data = await requestJson<SessionResponse>(
      '/api/auth/login',
      { method: 'POST', body: JSON.stringify(input) },
      'No fue posible iniciar sesión. Inténtalo de nuevo.',
    );
    return data.account;
  },

  async logInDemo(): Promise<Account> {
    const data = await requestJson<SessionResponse>(
      '/api/auth/demo',
      { method: 'POST' },
      'No fue posible iniciar la sesión demo.',
    );
    return data.account;
  },

  async logOut(): Promise<void> {
    const response = await fetch('/api/auth/logout', { method: 'POST' });
    if (!response.ok && response.status !== 204) {
      throw await parseErrorResponse(response, 'No fue posible cerrar la sesión.');
    }
  },

  async sendEmailCode(): Promise<{ devCode?: string }> {
    return requestJson<CodeIssuedResponse>(
      '/api/auth/email-verification',
      { method: 'POST' },
      'No fue posible enviar el código de verificación.',
    );
  },

  async confirmEmail(code: string): Promise<Account> {
    const payload: ConfirmEmailRequest = { code };
    const data = await requestJson<SessionResponse>(
      '/api/auth/email-verification/confirm',
      { method: 'POST', body: JSON.stringify(payload) },
      'No fue posible verificar el correo.',
    );
    return data.account;
  },

  async startPinRecovery(identifier: string): Promise<{ devCode?: string }> {
    const payload: StartPinRecoveryRequest = { identifier };
    return requestJson<CodeIssuedResponse>(
      '/api/auth/pin-recovery',
      { method: 'POST', body: JSON.stringify(payload) },
      'No fue posible iniciar la recuperación del PIN.',
    );
  },

  async confirmPinRecovery(input: ConfirmPinRecoveryRequest): Promise<Account> {
    const data = await requestJson<SessionResponse>(
      '/api/auth/pin-recovery/confirm',
      { method: 'POST', body: JSON.stringify(input) },
      'No fue posible actualizar tu PIN.',
    );
    return data.account;
  },
};
