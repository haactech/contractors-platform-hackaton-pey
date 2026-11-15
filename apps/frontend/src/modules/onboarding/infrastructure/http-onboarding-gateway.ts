import type {
  ApiErrorResponse,
  CreateOnboardingRequest,
  OnboardingDto,
  UpdateOnboardingProfileRequest,
} from '@platform/contracts';
import { OnboardingError, type OnboardingGateway } from '../application/onboarding-gateway';

async function parseErrorResponse(response: Response, fallbackMessage: string): Promise<OnboardingError> {
  try {
    const error: ApiErrorResponse = await response.json();
    return new OnboardingError(error.message, error.errors, response.status, error.code);
  } catch {
    return new OnboardingError(fallbackMessage, {}, response.status);
  }
}

export const httpOnboardingGateway: OnboardingGateway = {
  async create(input) {
    const payload: CreateOnboardingRequest = { ...input, client: { ...input.client } };
    const response = await fetch('/api/onboarding', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
      body: JSON.stringify(payload),
    });
    if (!response.ok) {
      throw await parseErrorResponse(response, 'No fue posible guardar el registro. Inténtalo de nuevo.');
    }
    const data: OnboardingDto = await response.json();
    return { ...data, client: { ...data.client } };
  },

  async update(id, input: UpdateOnboardingProfileRequest) {
    const response = await fetch(`/api/onboarding/${encodeURIComponent(id)}`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
      body: JSON.stringify(input),
    });
    if (!response.ok) {
      throw await parseErrorResponse(response, 'No fue posible actualizar tu perfil. Inténtalo de nuevo.');
    }
    const data: OnboardingDto = await response.json();
    return { ...data, client: { ...data.client } };
  },

  async find(id, signal) {
    const trimmed = id.trim();
    if (!trimmed) {
      throw new OnboardingError('Ingresa un folio para consultar.');
    }
    const response = await fetch(`/api/onboarding/${encodeURIComponent(trimmed)}`, {
      headers: { Accept: 'application/json' },
      signal,
    });
    if (!response.ok) {
      throw await parseErrorResponse(response, 'No fue posible consultar el registro. Inténtalo de nuevo.');
    }
    const data: OnboardingDto = await response.json();
    return { ...data, client: { ...data.client } };
  },
};
