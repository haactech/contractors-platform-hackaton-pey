import { useEffect, useId, useRef, useState, type FormEvent } from 'react';
import { AuthError, type AuthGateway } from '../application/auth-gateway';
import { isValidPin, PIN_LENGTH, type Account, type AuthConfig } from '../domain/auth';
import { AuthLayout } from './AuthLayout';

interface VerifyEmailFormProps {
  readonly gateway: AuthGateway;
  readonly account: Account;
  readonly config: AuthConfig;
  readonly onVerified: (account: Account) => void;
  readonly onContinueWithoutVerify?: () => void;
  readonly onSignOut: () => void;
}

export function VerifyEmailForm({
  gateway,
  account,
  config,
  onVerified,
  onContinueWithoutVerify,
  onSignOut,
}: VerifyEmailFormProps) {
  const [code, setCode] = useState('');
  const [codeError, setCodeError] = useState<string | null>(null);
  const [formError, setFormError] = useState<string | null>(null);
  const [devCode, setDevCode] = useState<string | null>(null);
  const [cooldown, setCooldown] = useState(0);
  const [submitting, setSubmitting] = useState<'confirm' | 'resend' | null>(null);
  const [statusMessage, setStatusMessage] = useState<string | null>(null);

  const requestedInitial = useRef(false);
  const baseId = useId();

  useEffect(() => {
    if (cooldown <= 0) return;
    const timer = window.setInterval(() => {
      setCooldown((prev) => (prev > 1 ? prev - 1 : 0));
    }, 1000);
    return () => window.clearInterval(timer);
  }, [cooldown]);

  useEffect(() => {
    if (requestedInitial.current || account.emailVerified) return;
    requestedInitial.current = true;
    gateway
      .sendEmailCode()
      .then((res) => {
        if (res.devCode) setDevCode(res.devCode);
        setCooldown(60);
      })
      .catch((error: unknown) => {
        if (error instanceof AuthError && error.status === 429) {
          setCooldown(error.retryAfterSeconds ?? 60);
        }
      });
  }, [gateway, account.emailVerified]);

  async function handleConfirm(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (submitting) return;
    if (!isValidPin(code)) {
      setCodeError('Ingresa el código de 6 dígitos.');
      return;
    }

    setSubmitting('confirm');
    setCodeError(null);
    setFormError(null);
    try {
      const updated = await gateway.confirmEmail(code);
      onVerified(updated);
    } catch (error) {
      if (error instanceof AuthError) {
        setCodeError(error.fields.code ?? error.message);
      } else {
        setFormError('No fue posible verificar el código.');
      }
    } finally {
      setSubmitting(null);
    }
  }

  async function handleResend() {
    if (submitting || cooldown > 0) return;
    setSubmitting('resend');
    setFormError(null);
    setStatusMessage(null);
    try {
      const res = await gateway.sendEmailCode();
      if (res.devCode) setDevCode(res.devCode);
      setCooldown(60);
      setStatusMessage('Enviamos un código nuevo a tu correo.');
    } catch (error) {
      if (error instanceof AuthError && error.status === 429) {
        setCooldown(error.retryAfterSeconds ?? 60);
        setFormError(error.message);
      } else {
        setFormError('No fue posible reenviar el código.');
      }
    } finally {
      setSubmitting(null);
    }
  }

  const codeId = `${baseId}-code`;

  return (
    <AuthLayout>
      <header className="auth-header">
        <h1>Verifica tu correo</h1>
        <p className="auth-subtitle">
          Enviamos un código de 6 dígitos a <strong>{account.email}</strong>. Vence en 15 minutos.
        </p>
      </header>

      {devCode && (
        <div className="auth-dev-chip" role="status">
          <span>Código de prueba (modo demo):</span>
          <strong className="auth-mono">{devCode}</strong>
        </div>
      )}

      <form className="auth-form" onSubmit={handleConfirm} noValidate>
        <div className="auth-field">
          <label htmlFor={codeId}>
            Código de 6 dígitos
            <span className="auth-required" aria-hidden="true"> *</span>
          </label>
          <input
            id={codeId}
            name="code"
            className="auth-input auth-pin-input"
            type="text"
            inputMode="numeric"
            pattern="[0-9]{6}"
            maxLength={PIN_LENGTH}
            autoComplete="one-time-code"
            placeholder="123456"
            value={code}
            disabled={submitting !== null}
            aria-required="true"
            aria-invalid={Boolean(codeError)}
            aria-describedby={codeError ? `${codeId}-error` : undefined}
            onChange={(event) => {
              setCode(event.target.value.replace(/\D/g, '').slice(0, PIN_LENGTH));
              setCodeError(null);
              setFormError(null);
            }}
          />
          {codeError && (
            <small id={`${codeId}-error`} role="alert" className="auth-error">
              {codeError}
            </small>
          )}
        </div>

        {statusMessage && (
          <p role="status" className="auth-status">
            {statusMessage}
          </p>
        )}

        {formError && (
          <p role="alert" className="auth-alert">
            {formError}
          </p>
        )}

        <button
          type="submit"
          className="auth-btn-primary"
          disabled={submitting !== null}
          aria-busy={submitting === 'confirm'}
        >
          {submitting === 'confirm' ? 'Verificando…' : 'Verificar correo'}
        </button>

        <button
          type="button"
          className="auth-btn-secondary"
          disabled={submitting !== null || cooldown > 0}
          aria-busy={submitting === 'resend'}
          onClick={handleResend}
        >
          {cooldown > 0 ? `Reenviar código (${cooldown} s)` : 'Reenviar código'}
        </button>
      </form>

      <div className="auth-footer-actions">
        {!config.emailVerificationRequired && onContinueWithoutVerify && (
          <button type="button" className="auth-link-btn" onClick={onContinueWithoutVerify}>
            Continuar sin verificar (modo demo)
          </button>
        )}
        <button type="button" className="auth-link-btn" onClick={onSignOut}>
          Cerrar sesión
        </button>
      </div>
    </AuthLayout>
  );
}
