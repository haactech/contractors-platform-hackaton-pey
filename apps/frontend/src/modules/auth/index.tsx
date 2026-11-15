import { httpAuthGateway } from './infrastructure/http-auth-gateway';
import { LoginForm } from './presentation/LoginForm';
import { RecoverPinForm } from './presentation/RecoverPinForm';
import { SignUpForm } from './presentation/SignUpForm';
import { VerifyEmailForm } from './presentation/VerifyEmailForm';
import './presentation/auth.css';

import type { Account, AuthConfig } from './domain/auth';

export type { Account, AuthConfig } from './domain/auth';
export { AuthError } from './application/auth-gateway';
export const authGateway = httpAuthGateway;

export function LoginScreen(props: {
  readonly config: AuthConfig;
  readonly onAuthenticated: (account: Account) => void;
  readonly onGoToSignUp: () => void;
  readonly onGoToRecover: () => void;
}) {
  return <LoginForm gateway={httpAuthGateway} {...props} />;
}

export function SignUpScreen(props: {
  readonly onAuthenticated: (account: Account) => void;
  readonly onGoToLogin: () => void;
}) {
  return <SignUpForm gateway={httpAuthGateway} {...props} />;
}

export function RecoverPinScreen(props: {
  readonly config: AuthConfig;
  readonly onAuthenticated: (account: Account) => void;
  readonly onGoToLogin: () => void;
}) {
  return <RecoverPinForm gateway={httpAuthGateway} {...props} />;
}

export function VerifyEmailScreen(props: {
  readonly account: Account;
  readonly config: AuthConfig;
  readonly onVerified: (account: Account) => void;
  readonly onContinueWithoutVerify?: () => void;
  readonly onSignOut: () => void;
}) {
  return <VerifyEmailForm gateway={httpAuthGateway} {...props} />;
}
