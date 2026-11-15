import { useId, useRef, useState, type FormEvent } from 'react';
import { OnboardingError, type OnboardingGateway } from '../application/onboarding-gateway';
import {
  BUSINESS_ACTIVITIES,
  ONBOARDING_STEP_FIELDS,
  OTHER_BUSINESS_ACTIVITY,
  findFirstErrorStep,
  formatMxPhone,
  isValidClabe,
  mxPhoneDigits,
  normalizeClabe,
  validateOnboarding,
  validateOnboardingStep,
  type FieldErrors,
  type OnboardingInput,
  type OnboardingStepNumber,
} from '../domain/onboarding';
import './onboarding.css';

interface FieldConfig {
  readonly name: string;
  readonly label: string;
  readonly optional?: boolean;
  readonly fullWidth?: boolean;
  readonly hint?: string;
  readonly type?: string;
  readonly autoComplete?: string;
  readonly inputMode?: 'numeric' | 'tel' | 'email';
  readonly maxLength?: number;
  readonly placeholder?: string;
  /** Renders a select with these options instead of a text input. */
  readonly options?: readonly string[];
  /** Masks the input as a Mexican phone: +52 prefix, "55 1234 5678". */
  readonly mask?: 'mxPhone';
}

interface StepMeta {
  readonly step: OnboardingStepNumber;
  readonly shortLabel: string;
  readonly title: string;
  readonly description: string;
  readonly fields: readonly FieldConfig[];
}

/* `client.*` is the business's first customer (docs/workflow.md, Stage 1),
 * not the person registering. */
const STEPS: readonly StepMeta[] = [
  {
    step: 1,
    shortLabel: 'Tus datos',
    title: '1. ¿Quién eres y dónde recibes tus pagos?',
    description:
      'Necesitamos tus datos básicos y tu CLABE para activar tu cuenta de cobro y depositar directo a tu banco.',
    fields: [
      {
        name: 'name',
        label: 'Tu nombre completo',
        autoComplete: 'name',
        placeholder: 'Ej. Ana García López',
        hint: 'Como aparece en tu identificación o cuenta bancaria.',
      },
      {
        name: 'profession',
        label: 'Tu profesión u oficio',
        placeholder: 'Ej. Electricista, maestra de inglés',
        hint: 'Ayuda a tus clientes a reconocer rápido el servicio.',
      },
      {
        name: 'clabe',
        label: 'CLABE para recibir tus pagos',
        fullWidth: true,
        inputMode: 'numeric',
        maxLength: 22,
        placeholder: '18 dígitos de tu cuenta bancaria',
        hint: 'Obligatoria. El dinero llega directo a esta cuenta; nosotros nunca lo tocamos.',
      },
    ],
  },
  {
    step: 2,
    shortLabel: 'Tu negocio (opcional)',
    title: '2. Datos de tu negocio (opcional)',
    description:
      'Si trabajas por tu cuenta o aún no tienes razón social o RFC, puedes omitir este paso y editarlo después en tu perfil.',
    fields: [
      {
        name: 'businessName',
        label: 'Nombre del negocio',
        optional: true,
        autoComplete: 'organization',
        placeholder: 'Ej. Servicios Eléctricos García',
      },
      {
        name: 'businessActivity',
        label: 'Giro del negocio',
        optional: true,
        options: [...BUSINESS_ACTIVITIES, OTHER_BUSINESS_ACTIVITY],
      },
      {
        name: 'rfc',
        label: 'RFC',
        optional: true,
        maxLength: 13,
        placeholder: 'Ej. GALA900101XXX',
        hint: '12 o 13 caracteres (opcional).',
      },
      {
        name: 'taxRegime',
        label: 'Régimen fiscal',
        optional: true,
        placeholder: 'Ej. RESICO',
      },
    ],
  },
  {
    step: 3,
    shortLabel: 'Primer cliente',
    title: '3. Registra a tu primer cliente',
    description: 'La persona a la que le harás tu primer cobro. Podrás agregar más clientes después.',
    fields: [
      {
        name: 'client.name',
        label: 'Nombre de tu cliente',
        fullWidth: true,
        placeholder: 'Ej. Carlos Mendoza',
      },
      {
        name: 'client.phone',
        label: 'Teléfono de tu cliente',
        type: 'tel',
        inputMode: 'tel',
        autoComplete: 'tel-national',
        mask: 'mxPhone',
        placeholder: '55 1234 5678',
        hint: '10 dígitos. Le enviaremos el cobro por WhatsApp.',
      },
      {
        name: 'client.email',
        label: 'Email de tu cliente',
        type: 'email',
        inputMode: 'email',
        placeholder: 'Ej. carlos@empresa.mx',
      },
    ],
  },
];

const VALUE_PROPOSITIONS = [
  {
    title: '0% comisión por cobro CoDi',
    detail: 'Recibe el monto íntegro de cada servicio sin porcentajes escondidos.',
    icon: 'percent',
  },
  {
    title: 'Directo a tu cuenta CLABE',
    detail: 'Tus clientes transfieren directo a tu banco; nunca retenemos fondos.',
    icon: 'shield',
  },
  {
    title: 'Enlaces y recordatorios claros',
    detail: 'Comparte cobros con QR o liga y mantén ordenado tu historial desde hoy.',
    icon: 'bolt',
  },
] as const;

type Phase = 'welcome' | OnboardingStepNumber;

type SubmissionState =
  | { status: 'idle' | 'saving' }
  | { status: 'error'; message: string }
  | { status: 'success'; id: string };

const INITIAL_INPUT: OnboardingInput = {
  name: '',
  profession: '',
  client: { name: '', phone: '', email: '' },
  taxRegime: '',
  businessName: '',
  businessActivity: '',
  rfc: '',
  clabe: '',
};

function maskClabe(clabe: string): string {
  const digits = normalizeClabe(clabe);
  if (digits.length < 4) return 'Sin capturar';
  return `•••• ${digits.slice(-4)}`;
}

function getFieldValue(input: OnboardingInput, name: string): string {
  if (name === 'client.name') return input.client.name;
  if (name === 'client.phone') return formatMxPhone(input.client.phone);
  if (name === 'client.email') return input.client.email;
  if (name === 'taxRegime') return input.taxRegime ?? '';
  return String(input[name as keyof Omit<OnboardingInput, 'client' | 'taxRegime'>] ?? '');
}

function setFieldValue(input: OnboardingInput, name: string, rawValue: string): OnboardingInput {
  if (name === 'client.name') return { ...input, client: { ...input.client, name: rawValue } };
  if (name === 'client.phone') return { ...input, client: { ...input.client, phone: mxPhoneDigits(rawValue) } };
  if (name === 'client.email') return { ...input, client: { ...input.client, email: rawValue } };
  if (name === 'rfc') return { ...input, rfc: rawValue.toUpperCase() };
  if (name === 'taxRegime') return { ...input, taxRegime: rawValue };
  return { ...input, [name]: rawValue };
}

function normalizePayload(input: OnboardingInput): OnboardingInput {
  const taxRegime = input.taxRegime?.trim();
  return {
    name: input.name.trim(),
    profession: input.profession.trim(),
    client: {
      name: input.client.name.trim(),
      phone: mxPhoneDigits(input.client.phone),
      email: input.client.email.trim(),
    },
    businessName: input.businessName.trim(),
    businessActivity: input.businessActivity.trim(),
    rfc: input.rfc.trim().toUpperCase(),
    clabe: normalizeClabe(input.clabe.trim()),
    ...(taxRegime ? { taxRegime } : {}),
  };
}

function FeatureIcon({ kind }: { kind: 'percent' | 'shield' | 'bolt' }) {
  if (kind === 'shield') {
    return (
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
        <path d="M12 3l7 3v6c0 4.5-3 8.2-7 9-4-.8-7-4.5-7-9V6l7-3z" strokeLinecap="round" strokeLinejoin="round" />
        <path d="M9.5 12l1.8 1.8 3.7-3.8" strokeLinecap="round" strokeLinejoin="round" />
      </svg>
    );
  }
  if (kind === 'bolt') {
    return (
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
        <path d="M13 2L4 14h7l-1 8 9-12h-7l1-8z" strokeLinecap="round" strokeLinejoin="round" />
      </svg>
    );
  }
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
      <circle cx="9" cy="9" r="2" />
      <circle cx="15" cy="15" r="2" />
      <path d="M16 8L8 16" strokeLinecap="round" />
      <circle cx="12" cy="12" r="9" />
    </svg>
  );
}

function ArrowRightIcon() {
  return (
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" aria-hidden="true">
      <path d="M5 12h14" strokeLinecap="round" />
      <path d="M13 6l6 6-6 6" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

function CheckIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" aria-hidden="true">
      <path d="M5 13l4 4L19 7" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

export function OnboardingForm({
  gateway,
  onCreated,
  onAlreadyOnboarded,
  onAuthInvalid,
}: {
  gateway: OnboardingGateway;
  onCreated?: (id: string) => void;
  onAlreadyOnboarded?: () => void;
  onAuthInvalid?: () => void;
}) {
  const [phase, setPhase] = useState<Phase>('welcome');
  const [formState, setFormState] = useState<OnboardingInput>(INITIAL_INPUT);
  const [activityChoice, setActivityChoice] = useState('');
  const [errors, setErrors] = useState<FieldErrors>({});
  const [submission, setSubmission] = useState<SubmissionState>({ status: 'idle' });

  const formRef = useRef<HTMLFormElement>(null);
  const pending = useRef(false);
  const baseId = useId();

  function focusField(fieldName?: string) {
    if (!fieldName) return;
    requestAnimationFrame(() => {
      const target =
        fieldName === 'businessActivity' && activityChoice === OTHER_BUSINESS_ACTIVITY
          ? 'businessActivityOther'
          : fieldName;
      const element = formRef.current?.elements.namedItem(target) as HTMLElement | null;
      element?.focus();
    });
  }

  function clearFieldError(name: string) {
    setErrors((prev) => {
      if (!prev[name]) return prev;
      const copy = { ...prev };
      delete copy[name];
      return copy;
    });
    if (submission.status === 'error') {
      setSubmission({ status: 'idle' });
    }
  }

  function handleFieldChange(name: string, nextValue: string) {
    setFormState((prev) => setFieldValue(prev, name, nextValue));
    clearFieldError(name);
  }

  function handleActivitySelect(selected: string) {
    setActivityChoice(selected);
    const resolved = selected === OTHER_BUSINESS_ACTIVITY ? '' : selected;
    setFormState((prev) => ({ ...prev, businessActivity: resolved }));
    clearFieldError('businessActivity');
  }

  function handleStartWizard() {
    setSubmission({ status: 'idle' });
    setPhase(1);
    focusField(ONBOARDING_STEP_FIELDS[1][0]);
  }

  function handleSelectStep(targetStep: OnboardingStepNumber) {
    if (phase === 'welcome') return;
    if (targetStep <= phase) {
      setSubmission({ status: 'idle' });
      setPhase(targetStep);
      return;
    }
    const normalized = normalizePayload(formState);
    const currentStepErrors = validateOnboardingStep(normalized, ONBOARDING_STEP_FIELDS[phase]);
    if (Object.keys(currentStepErrors).length) {
      setErrors((prev) => ({ ...prev, ...currentStepErrors }));
      setSubmission({ status: 'error', message: 'Completa los campos requeridos antes de avanzar.' });
      focusField(Object.keys(currentStepErrors)[0]);
      return;
    }
    setSubmission({ status: 'idle' });
    setPhase(targetStep);
    focusField(ONBOARDING_STEP_FIELDS[targetStep][0]);
  }

  function handleNextStep() {
    if (phase === 'welcome' || phase === 3) return;
    const normalized = normalizePayload(formState);
    const stepErrors = validateOnboardingStep(normalized, ONBOARDING_STEP_FIELDS[phase]);
    if (Object.keys(stepErrors).length) {
      setErrors((prev) => ({ ...prev, ...stepErrors }));
      setSubmission({ status: 'error', message: 'Revisa los campos indicados para continuar.' });
      focusField(Object.keys(stepErrors)[0]);
      return;
    }
    const nextStep = (phase + 1) as OnboardingStepNumber;
    setSubmission({ status: 'idle' });
    setPhase(nextStep);
    focusField(ONBOARDING_STEP_FIELDS[nextStep][0]);
  }

  function handleSkipStepTwo() {
    if (phase !== 2 || submission.status === 'saving') return;
    setFormState((prev) => ({
      ...prev,
      businessName: '',
      businessActivity: '',
      rfc: '',
      taxRegime: '',
    }));
    setActivityChoice('');
    setErrors((prev) => {
      const next = { ...prev };
      for (const field of ONBOARDING_STEP_FIELDS[2]) {
        delete next[field];
      }
      return next;
    });
    setSubmission({ status: 'idle' });
    setPhase(3);
    focusField(ONBOARDING_STEP_FIELDS[3][0]);
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (phase !== 3) {
      handleNextStep();
      return;
    }
    if (pending.current) return;

    const payload = normalizePayload(formState);
    const nextErrors = validateOnboarding(payload);
    setErrors(nextErrors);

    if (Object.keys(nextErrors).length) {
      const errorStep = findFirstErrorStep(nextErrors);
      setPhase(errorStep);
      setSubmission({ status: 'error', message: 'Revisa los campos indicados antes de finalizar.' });
      const firstErrorKey = ONBOARDING_STEP_FIELDS[errorStep].find((key) => Boolean(nextErrors[key])) ?? Object.keys(nextErrors)[0];
      focusField(firstErrorKey);
      return;
    }

    pending.current = true;
    setSubmission({ status: 'saving' });
    try {
      const onboarding = await gateway.create(payload);
      setSubmission({ status: 'success', id: onboarding.id });
      onCreated?.(onboarding.id);
    } catch (error) {
      if (error instanceof OnboardingError) {
        if (error.status === 409) {
          onAlreadyOnboarded?.();
          return;
        }
        if (error.status === 401 || error.status === 403) {
          onAuthInvalid?.();
          return;
        }
        const fieldKeys = Object.keys(error.fields);
        if (fieldKeys.length) {
          setErrors(error.fields);
          const errorStep = findFirstErrorStep(error.fields);
          setPhase(errorStep);
          const firstField = ONBOARDING_STEP_FIELDS[errorStep].find((key) => Boolean(error.fields[key])) ?? fieldKeys[0];
          focusField(firstField);
        }
        setSubmission({ status: 'error', message: error.message });
      } else {
        setSubmission({ status: 'error', message: 'No fue posible guardar el registro. Inténtalo de nuevo.' });
      }
    } finally {
      pending.current = false;
    }
  }

  const normalizedClabeDigits = normalizeClabe(formState.clabe).replace(/\D/g, '');
  const clabeReady = isValidClabe(normalizedClabeDigits);
  const currentStepMeta = phase === 'welcome' ? null : STEPS[phase - 1];

  return (
    <section className="ob-section" aria-label="Registro de cuenta">
      {phase === 'welcome' ? (
        <div className="ob-welcome">
          <div className="ob-eyebrow">
            <FeatureIcon kind="shield" />
            <span>Registro fintech · Liquidación directa a tu CLABE</span>
          </div>

          <header className="ob-welcome-head">
            <h2>Activa tu cuenta de cobros sin comisiones ocultas</h2>
            <p className="ob-subtitle">
              Pensado para contratistas e independientes: registra tu perfil en 3 pasos cortos y recibe cada pago directo en tu banco.
            </p>
          </header>

          <div className="ob-value-grid">
            {VALUE_PROPOSITIONS.map((item) => (
              <article key={item.title} className="ob-value-card">
                <div className="ob-value-icon">
                  <FeatureIcon kind={item.icon} />
                </div>
                <h3>{item.title}</h3>
                <p>{item.detail}</p>
              </article>
            ))}
          </div>

          <ol className="ob-roadmap" aria-label="Fases del registro">
            {STEPS.map((step) => (
              <li key={step.step}>
                <span className="ob-roadmap-num">{step.step}</span>
                <span>{step.shortLabel}</span>
              </li>
            ))}
          </ol>

          <div className="ob-welcome-cta">
            <button type="button" className="ob-btn-primary" onClick={handleStartWizard}>
              <span>Comenzar registro</span>
              <ArrowRightIcon />
            </button>
          </div>
        </div>
      ) : (
        <>
          <div className="ob-stepper-wrap">
            <div className="ob-stepper-top">
              <span className="ob-step-counter">Paso {phase} de {STEPS.length}</span>
              <span className="ob-step-counter">{currentStepMeta?.shortLabel}</span>
            </div>
            <div
              className="ob-progress-track"
              role="progressbar"
              aria-label="Avance del registro"
              aria-valuemin={1}
              aria-valuemax={STEPS.length}
              aria-valuenow={phase}
            >
              <div className="ob-progress-fill" style={{ width: `${(phase / STEPS.length) * 100}%` }} />
            </div>
            <nav className="ob-steps-nav" aria-label="Fases del registro">
              {STEPS.map((item) => {
                const isCurrent = item.step === phase;
                const isDone = item.step < phase;
                return (
                  <button
                    key={item.step}
                    type="button"
                    className={`ob-step-pill${isDone ? ' is-done' : ''}`}
                    aria-current={isCurrent ? 'step' : undefined}
                    disabled={submission.status === 'saving'}
                    onClick={() => handleSelectStep(item.step)}
                  >
                    <span className="ob-step-badge">{isDone ? <CheckIcon /> : item.step}</span>
                    <span>{item.shortLabel}</span>
                  </button>
                );
              })}
            </nav>
          </div>

          <form ref={formRef} onSubmit={handleSubmit} noValidate>
            {currentStepMeta && (
              <>
                <header className="ob-phase-head">
                  <h2>{currentStepMeta.title}</h2>
                  <p className="ob-subtitle">{currentStepMeta.description}</p>
                </header>

                <div className="onboarding-fields">
                  {currentStepMeta.fields.map((field) => {
                    const inputId = `${baseId}-${field.name}`;
                    const hintId = field.hint ? `${inputId}-hint` : undefined;
                    const errorId = errors[field.name] ? `${inputId}-error` : undefined;
                    const describedBy = [hintId, errorId].filter(Boolean).join(' ') || undefined;
                    const value = getFieldValue(formState, field.name);

                    return (
                      <div key={field.name} className={`form-field${field.fullWidth ? ' ob-field-span-2' : ''}`}>
                        <label htmlFor={inputId}>
                          {field.label}
                          {!field.optional && (
                            <span className="ob-required-mark" aria-hidden="true">
                              *
                            </span>
                          )}
                        </label>

                        {field.options ? (
                          <>
                            <select
                              id={inputId}
                              name={field.name}
                              required={!field.optional}
                              disabled={submission.status === 'saving'}
                              aria-invalid={Boolean(errors[field.name])}
                              aria-describedby={describedBy}
                              value={activityChoice}
                              onChange={(event) => handleActivitySelect(event.target.value)}
                            >
                              <option value="" disabled>
                                Elige una opción
                              </option>
                              {field.options.map((option) => (
                                <option key={option} value={option}>
                                  {option}
                                </option>
                              ))}
                            </select>
                            {activityChoice === OTHER_BUSINESS_ACTIVITY && (
                              <input
                                name={`${field.name}Other`}
                                aria-label="Escribe tu giro"
                                placeholder="Escribe tu giro"
                                maxLength={255}
                                value={formState.businessActivity}
                                disabled={submission.status === 'saving'}
                                aria-invalid={Boolean(errors[field.name])}
                                aria-describedby={errorId}
                                onChange={(event) => handleFieldChange(field.name, event.target.value)}
                                autoFocus
                              />
                            )}
                          </>
                        ) : field.mask === 'mxPhone' ? (
                          <div className="masked-input">
                            <span className="masked-input-prefix" aria-hidden="true">
                              +52
                            </span>
                            <input
                              id={inputId}
                              name={field.name}
                              type={field.type ?? 'tel'}
                              autoComplete={field.autoComplete}
                              inputMode={field.inputMode}
                              placeholder={field.placeholder}
                              value={value}
                              disabled={submission.status === 'saving'}
                              required={!field.optional}
                              aria-invalid={Boolean(errors[field.name])}
                              aria-describedby={describedBy}
                              onChange={(event) => handleFieldChange(field.name, event.target.value)}
                            />
                          </div>
                        ) : (
                          <input
                            id={inputId}
                            name={field.name}
                            type={field.type ?? 'text'}
                            autoComplete={field.autoComplete}
                            inputMode={field.inputMode}
                            maxLength={field.maxLength}
                            placeholder={field.placeholder}
                            value={value}
                            disabled={submission.status === 'saving'}
                            required={!field.optional}
                            aria-invalid={Boolean(errors[field.name])}
                            aria-describedby={describedBy}
                            onChange={(event) => handleFieldChange(field.name, event.target.value)}
                          />
                        )}

                        {field.name === 'clabe' && (
                          <span className={`ob-clabe-status${clabeReady ? ' is-valid' : ''}`} aria-live="polite">
                            {clabeReady
                              ? `CLABE válida · terminación ${maskClabe(normalizedClabeDigits)}`
                              : `${normalizedClabeDigits.length}/18 dígitos`}
                          </span>
                        )}
                        {field.hint && (
                          <small id={hintId} className="field-hint">
                            {field.hint}
                          </small>
                        )}
                        {errors[field.name] && (
                          <small id={errorId} role="alert" className="field-error">
                            {errors[field.name]}
                          </small>
                        )}
                      </div>
                    );
                  })}
                </div>
              </>
            )}

            {phase === 3 && (
              <aside className="ob-review-strip" aria-label="Resumen previo al envío">
                <h3>Confirmación rápida de tu cuenta</h3>
                <dl className="ob-record-grid">
                  <div>
                    <dt>Titular</dt>
                    <dd>{formState.name.trim() || 'Pendiente'}</dd>
                  </div>
                  <div>
                    <dt>Negocio</dt>
                    <dd>{formState.businessName.trim() || 'Pendiente'}</dd>
                  </div>
                  <div>
                    <dt>RFC</dt>
                    <dd>{formState.rfc.trim().toUpperCase() || 'Pendiente'}</dd>
                  </div>
                  <div>
                    <dt>CLABE</dt>
                    <dd>{maskClabe(formState.clabe)}</dd>
                  </div>
                </dl>
              </aside>
            )}

            {submission.status === 'error' && (
              <p role="alert" className="ob-alert">
                {submission.message}
              </p>
            )}

            {submission.status === 'success' && (
              <p role="status" className="ob-status">
                Registro guardado correctamente. Folio asignado: <span className="ob-folio-code">{submission.id}</span>
              </p>
            )}

            <footer className="ob-wizard-footer">
              <button
                type="button"
                className="ob-btn-secondary"
                disabled={submission.status === 'saving'}
                onClick={() => {
                  setSubmission({ status: 'idle' });
                  setPhase(phase === 1 ? 'welcome' : ((phase - 1) as OnboardingStepNumber));
                }}
              >
                {phase === 1 ? 'Volver a bienvenida' : 'Atrás'}
              </button>

              {phase < 3 ? (
                <div className="ob-footer-actions">
                  {phase === 2 && (
                    <button
                      type="button"
                      className="ob-btn-ghost"
                      disabled={submission.status === 'saving'}
                      onClick={handleSkipStepTwo}
                    >
                      Omitir este paso
                    </button>
                  )}
                  <button type="button" className="ob-btn-primary" onClick={handleNextStep}>
                    <span>Continuar</span>
                    <ArrowRightIcon />
                  </button>
                </div>
              ) : (
                <button type="submit" className="ob-btn-primary" disabled={submission.status === 'saving'}>
                  {submission.status === 'saving' ? 'Guardando registro...' : 'Finalizar registro'}
                </button>
              )}
            </footer>
          </form>
        </>
      )}
    </section>
  );
}
