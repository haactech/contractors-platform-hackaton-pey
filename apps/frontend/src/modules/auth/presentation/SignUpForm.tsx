import { useId, useRef, useState, type FormEvent } from 'react';
import { AuthError, type AuthGateway } from '../application/auth-gateway';
import {
  formatMxPhone,
  mxPhoneDigits,
  validateSignUp,
  type Account,
  type FieldErrors,
} from '../domain/auth';
import { AuthLayout } from './AuthLayout';
import { PinInput } from './PinInput';

interface SignUpFormProps {
  readonly gateway: AuthGateway;
  readonly onAuthenticated: (account: Account) => void;
  readonly onGoToLogin: () => void;
}

export function SignUpForm({ gateway, onAuthenticated, onGoToLogin }: SignUpFormProps) {
  const [email, setEmail] = useState('');
  const [phoneDigits, setPhoneDigits] = useState('');
  const [pin, setPin] = useState('');
  const [confirmPin, setConfirmPin] = useState('');
  const [errors, setErrors] = useState<FieldErrors>({});
  const [formError, setFormError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const formRef = useRef<HTMLFormElement>(null);
  const baseId = useId();

  function focusField(name: string) {
    requestAnimationFrame(() => {
      const element = formRef.current?.elements.namedItem(name) as HTMLElement | null;
      element?.focus();
    });
  }

  function clearError(field: string) {
    setErrors((prev) => {
      if (!prev[field]) return prev;
      const next = { ...prev };
      delete next[field];
      return next;
    });
    setFormError(null);
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (submitting) return;

    const validation = validateSignUp({ email, phoneDigits, pin, confirmPin });
    setErrors(validation);
    setFormError(null);
    if (Object.keys(validation).length > 0) {
      focusField(Object.keys(validation)[0]!);
      return;
    }

    setSubmitting(true);
    try {
      const account = await gateway.signUp({
        email: email.trim(),
        phone: `+52${phoneDigits}`,
        pin,
      });
      onAuthenticated(account);
    } catch (error) {
      if (error instanceof AuthError) {
        if (Object.keys(error.fields).length > 0) {
          setErrors(error.fields);
          focusField(Object.keys(error.fields)[0]!);
        }
        setFormError(error.message);
      } else {
        setFormError('No fue posible crear tu cuenta. Inténtalo de nuevo.');
      }
    } finally {
      setSubmitting(false);
    }
  }

  const emailId = `${baseId}-email`;
  const phoneId = `${baseId}-phone`;
  const emailHintId = `${emailId}-hint`;
  const phoneHintId = `${phoneId}-hint`;

  return (
    <AuthLayout>
      <header className="auth-header">
        <h1>Crea tu cuenta</h1>
        <p className="auth-subtitle">En 1 minuto creas tu acceso y después configuras tu negocio y CLABE.</p>
      </header>

      <form ref={formRef} className="auth-form" onSubmit={handleSubmit} noValidate>
        <div className="auth-field">
          <label htmlFor={emailId}>
            Correo electrónico
            <span className="auth-required" aria-hidden="true"> *</span>
          </label>
          <input
            id={emailId}
            name="email"
            className="auth-input"
            type="email"
            inputMode="email"
            autoComplete="email"
            placeholder="ana@correo.mx"
            value={email}
            disabled={submitting}
            aria-required="true"
            aria-invalid={Boolean(errors.email)}
            aria-describedby={[emailHintId, errors.email ? `${emailId}-error` : ''].filter(Boolean).join(' ')}
            onChange={(event) => {
              setEmail(event.target.value);
              clearError('email');
            }}
          />
          <small id={emailHintId} className="auth-hint">
            Te servirá para entrar, verificar tu cuenta y recuperar tu PIN.
          </small>
          {errors.email && (
            <small id={`${emailId}-error`} role="alert" className="auth-error">
              {errors.email}
            </small>
          )}
        </div>

        <div className="auth-field">
          <label htmlFor={phoneId}>
            Teléfono celular
            <span className="auth-required" aria-hidden="true"> *</span>
          </label>
          <div className="auth-masked-input">
            <span className="auth-masked-prefix" aria-hidden="true">+52</span>
            <input
              id={phoneId}
              name="phone"
              type="tel"
              inputMode="tel"
              autoComplete="tel-national"
              placeholder="55 1234 5678"
              value={formatMxPhone(phoneDigits)}
              disabled={submitting}
              aria-required="true"
              aria-invalid={Boolean(errors.phone)}
              aria-describedby={[phoneHintId, errors.phone ? `${phoneId}-error` : ''].filter(Boolean).join(' ')}
              onChange={(event) => {
                setPhoneDigits(mxPhoneDigits(event.target.value));
                clearError('phone');
              }}
            />
          </div>
          <small id={phoneHintId} className="auth-hint">
            10 dígitos. También podrás entrar con este número.
          </small>
          {errors.phone && (
            <small id={`${phoneId}-error`} role="alert" className="auth-error">
              {errors.phone}
            </small>
          )}
        </div>

        <PinInput
          id={`${baseId}-pin`}
          name="pin"
          label="Crea un PIN de 6 dígitos"
          value={pin}
          autoComplete="new-password"
          hint="Solo números (6 dígitos). Úsalo cada vez que entres."
          error={errors.pin}
          disabled={submitting}
          onChange={(nextPin) => {
            setPin(nextPin);
            clearError('pin');
            clearError('confirmPin');
          }}
        />

        <PinInput
          id={`${baseId}-confirm-pin`}
          name="confirmPin"
          label="Confirma tu PIN"
          value={confirmPin}
          autoComplete="new-password"
          error={errors.confirmPin}
          disabled={submitting}
          onChange={(nextPin) => {
            setConfirmPin(nextPin);
            clearError('confirmPin');
          }}
        />

        {formError && (
          <p role="alert" className="auth-alert">
            {formError}
          </p>
        )}

        <button type="submit" className="auth-btn-primary" disabled={submitting} aria-busy={submitting}>
          {submitting ? 'Creando cuenta…' : 'Crear cuenta'}
        </button>
      </form>

      <p className="auth-switch-row">
        ¿Ya tienes cuenta?{' '}
        <button type="button" className="auth-link-btn" onClick={onGoToLogin} disabled={submitting}>
          Entra aquí
        </button>
      </p>
    </AuthLayout>
  );
}
