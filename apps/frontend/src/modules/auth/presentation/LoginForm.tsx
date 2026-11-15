import { useId, useRef, useState, type FormEvent } from 'react';
import { AuthError, type AuthGateway } from '../application/auth-gateway';
import { validateLogIn, type Account, type AuthConfig, type FieldErrors } from '../domain/auth';
import { AuthLayout } from './AuthLayout';
import { PinInput } from './PinInput';

interface LoginFormProps {
  readonly gateway: AuthGateway;
  readonly config: AuthConfig;
  readonly onAuthenticated: (account: Account) => void;
  readonly onGoToSignUp: () => void;
  readonly onGoToRecover: () => void;
}

export function LoginForm({
  gateway,
  config,
  onAuthenticated,
  onGoToSignUp,
  onGoToRecover,
}: LoginFormProps) {
  const [identifier, setIdentifier] = useState('');
  const [pin, setPin] = useState('');
  const [errors, setErrors] = useState<FieldErrors>({});
  const [formError, setFormError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState<'login' | 'demo' | null>(null);

  const formRef = useRef<HTMLFormElement>(null);
  const baseId = useId();

  function focusField(name: string) {
    requestAnimationFrame(() => {
      const element = formRef.current?.elements.namedItem(name) as HTMLElement | null;
      element?.focus();
    });
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (submitting) return;

    const validation = validateLogIn({ identifier, pin });
    setErrors(validation);
    setFormError(null);
    if (Object.keys(validation).length > 0) {
      focusField(Object.keys(validation)[0]!);
      return;
    }

    setSubmitting('login');
    try {
      const account = await gateway.logIn({ identifier: identifier.trim(), pin });
      onAuthenticated(account);
    } catch (error) {
      if (error instanceof AuthError) {
        if (Object.keys(error.fields).length > 0) {
          setErrors(error.fields);
          focusField(Object.keys(error.fields)[0]!);
        }
        setFormError(error.message);
      } else {
        setFormError('No fue posible iniciar sesión. Inténtalo de nuevo.');
      }
    } finally {
      setSubmitting(null);
    }
  }

  async function handleDemoLogin() {
    if (submitting) return;
    setFormError(null);
    setErrors({});
    setSubmitting('demo');
    try {
      const account = await gateway.logInDemo();
      onAuthenticated(account);
    } catch (error) {
      setFormError(error instanceof AuthError ? error.message : 'No fue posible abrir la cuenta demo.');
    } finally {
      setSubmitting(null);
    }
  }

  const identifierId = `${baseId}-identifier`;
  const identifierErrorId = errors.identifier ? `${identifierId}-error` : undefined;

  return (
    <AuthLayout>
      <header className="auth-header">
        <h1>Entra a tu cuenta</h1>
        <p className="auth-subtitle">Usa tu correo o teléfono registrado y tu PIN de 6 dígitos.</p>
      </header>

      <form ref={formRef} className="auth-form" onSubmit={handleSubmit} noValidate>
        <div className="auth-field">
          <label htmlFor={identifierId}>
            Correo o teléfono
            <span className="auth-required" aria-hidden="true"> *</span>
          </label>
          <input
            id={identifierId}
            name="identifier"
            className="auth-input"
            type="text"
            autoComplete="username"
            placeholder="ana@correo.mx o 55 1234 5678"
            value={identifier}
            disabled={submitting !== null}
            aria-required="true"
            aria-invalid={Boolean(errors.identifier)}
            aria-describedby={identifierErrorId}
            onChange={(event) => {
              setIdentifier(event.target.value);
              setErrors((prev) => ({ ...prev, identifier: '' }));
              setFormError(null);
            }}
          />
          {errors.identifier && (
            <small id={identifierErrorId} role="alert" className="auth-error">
              {errors.identifier}
            </small>
          )}
        </div>

        <PinInput
          id={`${baseId}-pin`}
          name="pin"
          label="PIN de 6 dígitos"
          value={pin}
          autoComplete="current-password"
          error={errors.pin}
          disabled={submitting !== null}
          onChange={(nextPin) => {
            setPin(nextPin);
            setErrors((prev) => ({ ...prev, pin: '' }));
            setFormError(null);
          }}
        />

        <div className="auth-inline-links">
          <button type="button" className="auth-link-btn" onClick={onGoToRecover} disabled={submitting !== null}>
            ¿Olvidaste tu PIN?
          </button>
        </div>

        {formError && (
          <p role="alert" className="auth-alert">
            {formError}
          </p>
        )}

        <button
          type="submit"
          className="auth-btn-primary"
          disabled={submitting !== null}
          aria-busy={submitting === 'login'}
        >
          {submitting === 'login' ? 'Entrando…' : 'Entrar'}
        </button>
      </form>

      <p className="auth-switch-row">
        ¿Nuevo en Praxis?{' '}
        <button type="button" className="auth-link-btn" onClick={onGoToSignUp} disabled={submitting !== null}>
          Crea tu cuenta
        </button>
      </p>

      {config.demoAccount && (
        <div className="auth-demo-card" aria-label="Acceso rápido con cuenta demo">
          <div className="auth-demo-head">
            <span className="auth-demo-badge">Modo demo</span>
            <p className="auth-demo-creds">
              <code>{config.demoAccount.email}</code> · <code>{config.demoAccount.phone}</code> · PIN{' '}
              <code>{config.demoAccount.pin}</code>
            </p>
          </div>
          <p className="auth-demo-copy">
            Explora el tablero de cobros con un negocio de ejemplo precargado, sin llenar el registro.
          </p>
          <button
            type="button"
            className="auth-btn-secondary"
            disabled={submitting !== null}
            aria-busy={submitting === 'demo'}
            onClick={handleDemoLogin}
          >
            {submitting === 'demo' ? 'Abriendo demo…' : 'Probar con cuenta demo'}
          </button>
        </div>
      )}
    </AuthLayout>
  );
}
