import type { UpdateOnboardingProfileRequest } from '@platform/contracts';
import type { FieldErrors, Onboarding, OnboardingInput } from '../domain/onboarding';

export interface OnboardingGateway {
  create(input: OnboardingInput): Promise<Onboarding>;
  update(id: string, input: UpdateOnboardingProfileRequest): Promise<Onboarding>;
  find(id: string, signal?: AbortSignal): Promise<Onboarding>;
}

export class OnboardingError extends Error {
  constructor(
    message: string,
    readonly fields: FieldErrors = {},
    readonly status = 0,
    readonly code?: string,
  ) {
    super(message);
    this.name = 'OnboardingError';
  }
}
