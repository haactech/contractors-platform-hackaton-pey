import { useId, useRef, useState, type FormEvent } from 'react';
import { AuthError, type AuthGateway } from '../application/auth-gateway';
import {
  isValidIdentifier,
  PIN_LENGTH,
  validateRecoverPin,
  type Account,
  type AuthConfig,
  type FieldErrors,
} from '../domain/auth';
import { AuthLayout } from './AuthLayout';
import { PinInput } from './PinInput';

interface RecoverPinFormProps {
  readonly gateway: AuthGateway;
  readonly config: AuthConfig;
  readonly onAuthenticated: (account: Account) => void;
  readonly onGoToLogin: () => void;
}

export function RecoverPinForm({
  gateway,
  config,
  onAuthenticated,
  onGoToLogin,
}: RecoverPinFormProps) {
  const requiresCode = config.pinResetRequiresCode;
  const [step, setStep] = useState<'request' | 'confirm'>(requiresCode ? 'request' : 'confirm');
  const [identifier, setIdentifier] = useState('');
  const [code, setCode] = useState('');
  const [pin, setPin] = useState('');
  const [confirmPin, setConfirmPin] = useState('');
  const [devCode, setDevCode] = useState<string | null>(null);
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

  async function handleStart(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (submitting) return;
    if (!identifier.trim()) {
      setErrors({ identifier: 'Este campo es obligatorio.' });
      focusField('identifier');
      return;
    }
    if (!isValidIdentifier(identifier)) {
      setErrors({ identifier: 'Ingresa un correo o un teléfono de 10 dígitos.' });
      focusField('identifier');
      return;
    }

    setSubmitting(true);
    setErrors({});
    setFormError(null);
    try {
      const res = await gateway.startPinRecovery(identifier.trim());
      setDevCode(res.devCode ?? null);
      setStep('confirm');
    } catch (error) {
      if (error instanceof AuthError) {
        if (Object.keys(error.fields).length > 0) setErrors(error.fields);
        setFormError(error.message);
      } else {
        setFormError('No fue posible iniciar la recuperación.');
      }
    } finally {
      setSubmitting(false);
    }
  }

  async function handleConfirm(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (submitting) return;

    const validation = validateRecoverPin({ identifier, code, pin, confirmPin, requiresCode });
    setErrors(validation);
    setFormError(null);
    if (Object.keys(validation).length > 0) {
      focusField(Object.keys(validation)[0]!);
      return;
    }

    setSubmitting(true);
    try {
      const account = await gateway.confirmPinRecovery({
        identifier: identifier.trim(),
        ...(requiresCode ? { code: code.trim() } : {}),
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
        setFormError('No fue posible actualizar tu PIN.');
      }
    } finally {
      setSubmitting(false);
    }
  }

  const identifierId = `${baseId}-identifier`;
  const codeId = `${baseId}-code`;

  return (
    <AuthLayout>
      <header className="auth-header">
        <h1>Crea un PIN nuevo</h1>
        <p className="auth-subtitle">
          {requiresCode
            ? 'Te enviaremos un código de 6 dígitos al correo de tu cuenta.'
            : 'Ingresa tu correo o teléfono para asignar un PIN de 6 dígitos nuevo.'}
        </p>
      </header>

      {!requiresCode && (
        <p className="auth-demo-notice" role="note">
          Modo demo: temporalmente el PIN se cambia sin código de verificación por correo.
        </p>
      )}

      {requiresCode && step === 'request' ? (
        <form ref={formRef} className="auth-form" onSubmit={handleStart} noValidate>
          <div className="auth-field">
            <label htmlFor={identifierId}>
              Correo o teléfono de tu cuenta
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
              disabled={submitting}
              aria-required="true"
              aria-invalid={Boolean(errors.identifier)}
              aria-describedby={errors.identifier ? `${identifierId}-error` : undefined}
              onChange={(event) => {
                setIdentifier(event.target.value);
                clearError('identifier');
              }}
            />
            {errors.identifier && (
              <small id={`${identifierId}-error`} role="alert" className="auth-error">
                {errors.identifier}
              </small>
            )}
          </div>

          {formError && (
            <p role="alert" className="auth-alert">
              {formError}
            </p>
          )}

          <button type="submit" className="auth-btn-primary" disabled={submitting} aria-busy={submitting}>
            {submitting ? 'Enviando…' : 'Enviar código'}
          </button>
        </form>
      ) : (
        <form ref={formRef} className="auth-form" onSubmit={handleConfirm} noValidate>
          {requiresCode ? (
            <p className="auth-status" role="status">
              Si existe una cuenta con ese dato, enviamos un código de 6 dígitos a su correo.
            </p>
          ) : (
            <div className="auth-field">
              <label htmlFor={identifierId}>
                Correo o teléfono de tu cuenta
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
                disabled={submitting}
                aria-required="true"
                aria-invalid={Boolean(errors.identifier)}
                aria-describedby={errors.identifier ? `${identifierId}-error` : undefined}
                onChange={(event) => {
                  setIdentifier(event.target.value);
                  clearError('identifier');
                }}
              />
              {errors.identifier && (
                <small id={`${identifierId}-error`} role="alert" className="auth-error">
                  {errors.identifier}
                </small>
              )}
            </div>
          )}

          {devCode && (
            <div className="auth-dev-chip" role="status">
              <span>Código de prueba:</span>
              <strong className="auth-mono">{devCode}</strong>
            </div>
          )}

          {requiresCode && (
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
                disabled={submitting}
                aria-required="true"
                aria-invalid={Boolean(errors.code)}
                aria-describedby={errors.code ? `${codeId}-error` : undefined}
                onChange={(event) => {
                  setCode(event.target.value.replace(/\D/g, '').slice(0, PIN_LENGTH));
                  clearError('code');
                }}
              />
              {errors.code && (
                <small id={`${codeId}-error`} role="alert" className="auth-error">
                  {errors.code}
                </small>
              )}
            </div>
          )}

          <PinInput
            id={`${baseId}-pin`}
            name="pin"
            label="PIN nuevo de 6 dígitos"
            value={pin}
            autoComplete="new-password"
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
            label="Confirma tu PIN nuevo"
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
            {submitting ? 'Guardando PIN…' : 'Guardar nuevo PIN'}
          </button>

          {requiresCode && (
            <button
              type="button"
              className="auth-link-btn"
              disabled={submitting}
              onClick={() => {
                setStep('request');
                setCode('');
                setErrors({});
                setFormError(null);
              }}
            >
              Cambiar correo/teléfono o pedir otro código
            </button>
          )}
        </form>
      )}

      <p className="auth-switch-row">
        <button type="button" className="auth-link-btn" onClick={onGoToLogin} disabled={submitting}>
          Volver a iniciar sesión
        </button>
      </p>
    </AuthLayout>
  );
}
