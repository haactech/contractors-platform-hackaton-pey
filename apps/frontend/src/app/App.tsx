import { useCallback, useEffect, useState } from 'react';
import {
  LoginScreen,
  RecoverPinScreen,
  SignUpScreen,
  VerifyEmailScreen,
  authGateway,
  type Account,
  type AuthConfig,
} from '../modules/auth';
import { DashboardWorkspace } from '../modules/dashboard';
import { OnboardingError, OnboardingForm, updateOnboardingProfile } from '../modules/onboarding';
import { dashboardPath, navigate, ROUTES, useRoute, type Route } from './router';

type AuthState =
  | { readonly status: 'loading' }
  | { readonly status: 'anonymous' }
  | { readonly status: 'signedIn'; readonly account: Account };

const DEFAULT_CONFIG: AuthConfig = {
  emailVerificationRequired: false,
  pinResetRequiresCode: false,
  demoAccount: null,
};

function targetAfterAuth(account: Account, config: AuthConfig): string {
  if (config.emailVerificationRequired && !account.emailVerified) {
    return ROUTES.verifyEmail;
  }
  return account.contractorId ? dashboardPath(account.contractorId) : ROUTES.onboarding;
}

function resolveRoute(route: Route, auth: AuthState, config: AuthConfig): string | null {
  if (auth.status === 'loading') return null;

  if (auth.status === 'anonymous') {
    if (route.name === 'login' || route.name === 'signup' || route.name === 'recoverPin') {
      return null;
    }
    return ROUTES.login;
  }

  const { account } = auth;
  if (config.emailVerificationRequired && !account.emailVerified) {
    return route.name === 'verifyEmail' ? null : ROUTES.verifyEmail;
  }

  if (account.contractorId === null) {
    if (route.name === 'onboarding' || (route.name === 'verifyEmail' && !account.emailVerified)) {
      return null;
    }
    return ROUTES.onboarding;
  }

  if (route.name === 'dashboard' && route.contractorId === account.contractorId) {
    return null;
  }
  if (route.name === 'verifyEmail' && !account.emailVerified) {
    return null;
  }
  return dashboardPath(account.contractorId);
}

function Redirect({ to }: { to: string }) {
  useEffect(() => navigate(to, { replace: true }), [to]);
  return null;
}

export function App() {
  const route = useRoute();
  const [auth, setAuth] = useState<AuthState>({ status: 'loading' });
  const [config, setConfig] = useState<AuthConfig>(DEFAULT_CONFIG);

  const syncSession = useCallback(async (signal?: AbortSignal) => {
    try {
      const [nextConfig, account] = await Promise.all([
        authGateway.config(signal),
        authGateway.session(signal),
      ]);
      if (signal?.aborted) return;
      setConfig(nextConfig);
      setAuth(account ? { status: 'signedIn', account } : { status: 'anonymous' });
    } catch {
      if (signal?.aborted) return;
      setAuth({ status: 'anonymous' });
    }
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    void syncSession(controller.signal);
    return () => controller.abort();
  }, [syncSession]);

  function handleAuthenticated(account: Account) {
    setAuth({ status: 'signedIn', account });
    navigate(targetAfterAuth(account, config));
  }

  function handleRegistered(contractorId: string) {
    setAuth((prev) =>
      prev.status === 'signedIn'
        ? { status: 'signedIn', account: { ...prev.account, contractorId } }
        : prev,
    );
    navigate(dashboardPath(contractorId));
  }

  async function handleSignOut() {
    try {
      await authGateway.logOut();
    } catch {
      /* Clear client state even if server logout fails. */
    }
    setAuth({ status: 'anonymous' });
    navigate(ROUTES.login);
  }

  async function handleOnboardingOrDashboardAuthFailure() {
    try {
      const account = await authGateway.session();
      if (!account) {
        setAuth({ status: 'anonymous' });
        navigate(ROUTES.login, { replace: true });
        return;
      }
      setAuth({ status: 'signedIn', account });
      if (config.emailVerificationRequired && !account.emailVerified) {
        navigate(ROUTES.verifyEmail, { replace: true });
        return;
      }
      if (account.contractorId && route.name === 'onboarding') {
        navigate(dashboardPath(account.contractorId), { replace: true });
        return;
      }
      // 404 on own contractor: sign out so the user is not stuck in a broken loop.
      await handleSignOut();
    } catch {
      await handleSignOut();
    }
  }

  if (auth.status === 'loading') {
    return (
      <main>
        <div className="auth-loading" role="status">
          Cargando tu espacio…
        </div>
      </main>
    );
  }

  const redirectTo = resolveRoute(route, auth, config);
  if (redirectTo) {
    return <Redirect to={redirectTo} />;
  }

  switch (route.name) {
    case 'login':
      return (
        <main>
          <LoginScreen
            config={config}
            onAuthenticated={handleAuthenticated}
            onGoToSignUp={() => navigate(ROUTES.signup)}
            onGoToRecover={() => navigate(ROUTES.recoverPin)}
          />
        </main>
      );
    case 'signup':
      return (
        <main>
          <SignUpScreen
            onAuthenticated={handleAuthenticated}
            onGoToLogin={() => navigate(ROUTES.login)}
          />
        </main>
      );
    case 'recoverPin':
      return (
        <main>
          <RecoverPinScreen
            config={config}
            onAuthenticated={handleAuthenticated}
            onGoToLogin={() => navigate(ROUTES.login)}
          />
        </main>
      );
    case 'verifyEmail':
      if (auth.status !== 'signedIn') return <Redirect to={ROUTES.login} />;
      return (
        <main>
          <VerifyEmailScreen
            account={auth.account}
            config={config}
            onVerified={handleAuthenticated}
            onContinueWithoutVerify={() =>
              navigate(
                auth.account.contractorId
                  ? dashboardPath(auth.account.contractorId)
                  : ROUTES.onboarding,
              )
            }
            onSignOut={() => void handleSignOut()}
          />
        </main>
      );
    case 'onboarding':
      if (auth.status !== 'signedIn') return <Redirect to={ROUTES.login} />;
      return (
        <main>
          <div className="app-onboarding-topbar">
            <div>
              <h1>Plataforma de contratistas</h1>
              <p className="app-account-meta">Sesión iniciada como {auth.account.email}</p>
            </div>
            <div className="app-topbar-actions">
              {!auth.account.emailVerified && (
                <button
                  type="button"
                  className="app-topbar-link"
                  onClick={() => navigate(ROUTES.verifyEmail)}
                >
                  Verificar correo
                </button>
              )}
              <button
                type="button"
                className="app-topbar-btn"
                onClick={() => void handleSignOut()}
              >
                Cerrar sesión
              </button>
            </div>
          </div>
          <OnboardingForm
            onCreated={handleRegistered}
            onAlreadyOnboarded={() => void handleOnboardingOrDashboardAuthFailure()}
            onAuthInvalid={() => void handleOnboardingOrDashboardAuthFailure()}
          />
        </main>
      );
    case 'dashboard':
      if (auth.status !== 'signedIn') return <Redirect to={ROUTES.login} />;
      return (
        <main>
          <DashboardWorkspace
            key={route.contractorId}
            contractorId={route.contractorId}
            preloadDemo={auth.account.demo}
            onNotFound={() => void handleOnboardingOrDashboardAuthFailure()}
            onSignOut={() => void handleSignOut()}
            onUpdateProfile={async (patch) => {
              try {
                await updateOnboardingProfile(route.contractorId, patch);
              } catch (error) {
                if (
                  error instanceof OnboardingError &&
                  (error.status === 401 || error.status === 403 || error.status === 404)
                ) {
                  await handleOnboardingOrDashboardAuthFailure();
                  return;
                }
                throw error;
              }
            }}
          />
        </main>
      );
    case 'root':
    case 'notFound':
      return <Redirect to={ROUTES.login} />;
  }
}
