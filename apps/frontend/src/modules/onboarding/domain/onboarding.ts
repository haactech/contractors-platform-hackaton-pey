import {
  BUSINESS_ACTIVITIES,
  OTHER_BUSINESS_ACTIVITY,
  RFC_PATTERN,
  isValidClabe,
  normalizeClabe,
} from '../../../shared/business';
import { MX_PHONE_DIGITS } from '../../../shared/phone';

export {
  BUSINESS_ACTIVITIES,
  OTHER_BUSINESS_ACTIVITY,
  RFC_PATTERN,
  isValidClabe,
  normalizeClabe,
} from '../../../shared/business';
export { MX_PHONE_DIGITS, formatMxPhone, mxPhoneDigits } from '../../../shared/phone';

export interface OnboardingInput {
  name: string;
  profession: string;
  client: { name: string; phone: string; email: string };
  taxRegime?: string;
  businessName: string;
  businessActivity: string;
  rfc: string;
  clabe: string;
}

/** Public onboarding record from the API (full CLABE is write-only). */
export interface Onboarding {
  id: string;
  name: string;
  profession: string;
  client: { name: string; phone: string; email: string };
  taxRegime?: string;
  businessName: string;
  businessActivity: string;
  rfc: string;
  clabeLast4: string;
  codiStatus: 'not_registered' | 'PENDING' | 'PROCESSING' | 'ACTIVE' | 'ERROR';
  bankReady: boolean;
}

export type FieldErrors = Record<string, string>;

export function validateOnboarding(input: OnboardingInput): FieldErrors {
  const errors: FieldErrors = {};
  const required = {
    name: input.name,
    profession: input.profession,
    clabe: input.clabe,
    'client.name': input.client.name,
    'client.phone': input.client.phone,
    'client.email': input.client.email,
  };
  for (const [field, value] of Object.entries(required)) {
    if (!value.trim()) errors[field] = 'Este campo es obligatorio.';
    else if (value.length > 255) errors[field] = 'Usa un máximo de 255 caracteres.';
  }
  const optionalTexts = {
    businessName: input.businessName,
    businessActivity: input.businessActivity,
    taxRegime: input.taxRegime ?? '',
  };
  for (const [field, value] of Object.entries(optionalTexts)) {
    if (value.length > 255) errors[field] = 'Usa un máximo de 255 caracteres.';
  }
  if (input.client.email && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(input.client.email)) {
    errors['client.email'] = 'Escribe un email válido.';
  }
  if (input.client.phone && !/^\d{10}$/.test(input.client.phone)) {
    errors['client.phone'] = `Escribe un teléfono de ${MX_PHONE_DIGITS} dígitos.`;
  }
  if (input.rfc && !RFC_PATTERN.test(input.rfc)) {
    errors.rfc = 'Escribe un RFC de 12 o 13 caracteres con formato válido.';
  }
  if (input.clabe && !isValidClabe(input.clabe)) {
    errors.clabe = 'Escribe una CLABE válida de 18 dígitos.';
  }
  return errors;
}

export type OnboardingStepNumber = 1 | 2 | 3;

export const ONBOARDING_STEP_FIELDS: Record<OnboardingStepNumber, readonly string[]> = {
  1: ['name', 'profession', 'clabe'],
  2: ['businessName', 'businessActivity', 'rfc', 'taxRegime'],
  3: ['client.name', 'client.phone', 'client.email'],
};

/** Validates only the specified onboarding fields by reusing full domain rules. */
export function validateOnboardingStep(
  input: OnboardingInput,
  fields: readonly string[],
): FieldErrors {
  const allErrors = validateOnboarding(input);
  const scoped: FieldErrors = {};
  for (const field of fields) {
    if (allErrors[field]) {
      scoped[field] = allErrors[field];
    }
  }
  return scoped;
}

/** Returns the earliest step index (1-3) containing a validation error. */
export function findFirstErrorStep(errors: FieldErrors): OnboardingStepNumber {
  const keys = Object.keys(errors);
  for (const step of [1, 2, 3] as const) {
    if (ONBOARDING_STEP_FIELDS[step].some((field) => keys.includes(field))) {
      return step;
    }
  }
  return 1;
}
