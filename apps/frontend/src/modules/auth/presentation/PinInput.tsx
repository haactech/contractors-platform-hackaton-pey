import { useState } from 'react';
import { PIN_LENGTH } from '../domain/auth';

interface PinInputProps {
  readonly id: string;
  readonly name: string;
  readonly label: string;
  readonly value: string;
  readonly autoComplete: 'current-password' | 'new-password';
  readonly hint?: string;
  readonly error?: string;
  readonly disabled?: boolean;
  readonly onChange: (nextDigits: string) => void;
}

export function PinInput({
  id,
  name,
  label,
  value,
  autoComplete,
  hint,
  error,
  disabled,
  onChange,
}: PinInputProps) {
  const [visible, setVisible] = useState(false);
  const hintId = hint ? `${id}-hint` : undefined;
  const errorId = error ? `${id}-error` : undefined;
  const describedBy = [hintId, errorId].filter(Boolean).join(' ') || undefined;

  return (
    <div className="auth-field">
      <div className="auth-label-row">
        <label htmlFor={id}>
          {label}
          <span className="auth-required" aria-hidden="true"> *</span>
        </label>
        <button
          type="button"
          className="auth-toggle-btn"
          aria-pressed={visible}
          aria-label={visible ? `Ocultar ${label.toLowerCase()}` : `Mostrar ${label.toLowerCase()}`}
          disabled={disabled}
          onClick={() => setVisible((prev) => !prev)}
        >
          {visible ? 'Ocultar' : 'Ver'}
        </button>
      </div>
      <input
        id={id}
        name={name}
        className="auth-input auth-pin-input"
        type={visible ? 'text' : 'password'}
        inputMode="numeric"
        pattern="[0-9]{6}"
        maxLength={PIN_LENGTH}
        autoComplete={autoComplete}
        placeholder="••••••"
        value={value}
        disabled={disabled}
        aria-required="true"
        aria-invalid={Boolean(error)}
        aria-describedby={describedBy}
        onChange={(event) => onChange(event.target.value.replace(/\D/g, '').slice(0, PIN_LENGTH))}
      />
      {hint && (
        <small id={hintId} className="auth-hint">
          {hint}
        </small>
      )}
      {error && (
        <small id={errorId} role="alert" className="auth-error">
          {error}
        </small>
      )}
    </div>
  );
}
