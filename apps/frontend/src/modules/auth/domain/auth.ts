import type { AuthErrorCode } from '@platform/contracts';
import { MX_PHONE_DIGITS, formatMxPhone, mxPhoneDigits } from '../../../shared/phone';

export { MX_PHONE_DIGITS, formatMxPhone, mxPhoneDigits };

export const PIN_LENGTH = 6;

export interface Account {
  readonly id: string;
  readonly email: string;
  readonly phone: string;
  readonly contractorId: string | null;
  readonly emailVerified: boolean;
  readonly demo: boolean;
}

export interface DemoAccountInfo {
  readonly email: string;
  readonly phone: string;
  readonly pin: string;
}

export interface AuthConfig {
  readonly emailVerificationRequired: boolean;
  readonly pinResetRequiresCode: boolean;
  readonly demoAccount: DemoAccountInfo | null;
}

export type { AuthErrorCode };

export type FieldErrors = Record<string, string>;

export interface SignUpFormValues {
  readonly email: string;
  readonly phoneDigits: string;
  readonly pin: string;
  readonly confirmPin: string;
}

export interface LogInFormValues {
  readonly identifier: string;
  readonly pin: string;
}

export function isValidEmail(email: string): boolean {
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email.trim());
}

export function isValidPin(pin: string): boolean {
  return /^\d{6}$/.test(pin);
}

export function isValidIdentifier(raw: string): boolean {
  const trimmed = raw.trim();
  if (!trimmed) return false;
  if (trimmed.includes('@')) return isValidEmail(trimmed);
  return /^\d{10}$/.test(mxPhoneDigits(trimmed)) && /^[\d\s()+-]+$/.test(trimmed);
}

export function validateSignUp(values: SignUpFormValues): FieldErrors {
  const errors: FieldErrors = {};
  const email = values.email.trim();
  if (!email) errors.email = 'Este campo es obligatorio.';
  else if (email.length > 255) errors.email = 'Usa como máximo 255 caracteres.';
  else if (!isValidEmail(email)) errors.email = 'Ingresa un correo válido.';

  if (!values.phoneDigits) errors.phone = 'Este campo es obligatorio.';
  else if (values.phoneDigits.length !== MX_PHONE_DIGITS) {
    errors.phone = `Ingresa un teléfono de ${MX_PHONE_DIGITS} dígitos.`;
  }

  if (!values.pin) errors.pin = 'Este campo es obligatorio.';
  else if (!isValidPin(values.pin)) errors.pin = 'El PIN debe tener exactamente 6 dígitos.';

  if (!values.confirmPin) errors.confirmPin = 'Confirma tu PIN.';
  else if (values.pin !== values.confirmPin) errors.confirmPin = 'Los PIN no coinciden.';

  return errors;
}

export function validateLogIn(values: LogInFormValues): FieldErrors {
  const errors: FieldErrors = {};
  if (!values.identifier.trim()) errors.identifier = 'Este campo es obligatorio.';
  else if (!isValidIdentifier(values.identifier)) {
    errors.identifier = 'Ingresa un correo o un teléfono de 10 dígitos.';
  }
  if (!values.pin) errors.pin = 'Este campo es obligatorio.';
  else if (!isValidPin(values.pin)) errors.pin = 'El PIN debe tener exactamente 6 dígitos.';
  return errors;
}

export function validateRecoverPin(values: {
  identifier: string;
  code: string;
  pin: string;
  confirmPin: string;
  requiresCode: boolean;
}): FieldErrors {
  const errors: FieldErrors = {};
  if (!values.identifier.trim()) errors.identifier = 'Este campo es obligatorio.';
  else if (!isValidIdentifier(values.identifier)) {
    errors.identifier = 'Ingresa un correo o un teléfono de 10 dígitos.';
  }
  if (values.requiresCode) {
    if (!values.code.trim()) errors.code = 'Este campo es obligatorio.';
    else if (!isValidPin(values.code.trim())) errors.code = 'Ingresa el código de 6 dígitos.';
  }
  if (!values.pin) errors.pin = 'Este campo es obligatorio.';
  else if (!isValidPin(values.pin)) errors.pin = 'El PIN debe tener exactamente 6 dígitos.';

  if (!values.confirmPin) errors.confirmPin = 'Confirma tu PIN nuevo.';
  else if (values.pin !== values.confirmPin) errors.confirmPin = 'Los PIN no coinciden.';

  return errors;
}
