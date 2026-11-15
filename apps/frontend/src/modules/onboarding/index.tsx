import type { UpdateOnboardingProfileRequest } from '@platform/contracts';
import { OnboardingError } from './application/onboarding-gateway';
import { httpOnboardingGateway } from './infrastructure/http-onboarding-gateway';
import { OnboardingForm as Form } from './presentation/OnboardingForm';

export type { Onboarding, OnboardingInput } from './domain/onboarding';
export { OnboardingError };

export function updateOnboardingProfile(id: string, patch: UpdateOnboardingProfileRequest) {
  return httpOnboardingGateway.update(id, patch);
}

export function OnboardingForm({
  onCreated,
  onAlreadyOnboarded,
  onAuthInvalid,
}: {
  onCreated?: (id: string) => void;
  onAlreadyOnboarded?: () => void;
  onAuthInvalid?: () => void;
}) {
  return (
    <Form
      gateway={httpOnboardingGateway}
      onCreated={onCreated}
      onAlreadyOnboarded={onAlreadyOnboarded}
      onAuthInvalid={onAuthInvalid}
    />
  );
}
