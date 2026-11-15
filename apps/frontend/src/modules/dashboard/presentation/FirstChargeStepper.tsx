import { useId, useState } from 'react';
import type { CustomerRowDto, DashboardProfileDto, PaymentMethodDto } from '@platform/contracts';
import type { CreatedChargeResult, NewChargeInput } from '../application/dashboard-gateway';
import {
  CONCEPT_MAX_LENGTH,
  QUICK_AMOUNT_PESOS,
  QUICK_CONCEPTS,
  QUICK_PAYMENT_METHODS,
  formatDate,
  formatMxnCents,
  formatMxPhone,
  pesosToCents,
  todayInMexico,
} from '../domain/dashboard';
import { ArrowRightIcon, BoltIcon, CheckIcon, LinkIcon, PlusIcon, QrCodeIcon } from './DashboardIcons';

/* Shown on "Inicio" while the business has no charges: welcome, then a guided
 * flow to create the first charge. The customer captured in onboarding
 * (firstCustomer) is preselected. */

type StepId = 'welcome' | 'customer' | 'charge' | 'confirm';

const STEPS: readonly { readonly id: StepId; readonly label: string }[] = [
  { id: 'welcome', label: 'Bienvenida' },
  { id: 'customer', label: 'Cliente' },
  { id: 'charge', label: 'Cobro' },
  { id: 'confirm', label: 'Confirmar' },
];

const DEFAULT_DUE_IN_DAYS = 7;
const DEFAULT_AMOUNT_PESOS = '500';

function addDays(date: string, days: number): string {
  return new Date(Date.parse(`${date}T00:00:00Z`) + days * 86_400_000).toISOString().slice(0, 10);
}

interface FirstChargeStepperProps {
  readonly profile: DashboardProfileDto;
  readonly customers: readonly CustomerRowDto[];
  readonly customerPhones: Readonly<Record<string, string>>;
  readonly clabeLabel: string;
  readonly onCreateCharge: (input: NewChargeInput) => Promise<CreatedChargeResult>;
  readonly onCreated: (result: CreatedChargeResult) => void;
  readonly onRegisterCustomer: () => void;
}

export function FirstChargeStepper({
  profile,
  customers,
  customerPhones,
  clabeLabel,
  onCreateCharge,
  onCreated,
  onRegisterCustomer,
}: FirstChargeStepperProps) {
  const baseId = useId();
  const [step, setStep] = useState<StepId>('welcome');
  const firstCustomerId =
    customers.find((row) => row.name === profile.firstCustomer.name)?.customerId ?? customers[0]?.customerId ?? '';
  const [customerId, setCustomerId] = useState(firstCustomerId);
  const [concept, setConcept] = useState<string>(QUICK_CONCEPTS[0]);
  const [amount, setAmount] = useState(DEFAULT_AMOUNT_PESOS);
  const [dueDate, setDueDate] = useState(addDays(todayInMexico(), DEFAULT_DUE_IN_DAYS));
  const [method, setMethod] = useState<PaymentMethodDto>('PAYMENT_LINK');
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState(false);

  const stepIndex = STEPS.findIndex((item) => item.id === step);
  const customer = customers.find((row) => row.customerId === (customerId || firstCustomerId));
  const amountCents = pesosToCents(amount);
  const methodInfo = QUICK_PAYMENT_METHODS.find((item) => item.id === method);

  function goTo(next: StepId) {
    setErrors({});
    setStep(next);
  }

  function validateCharge(): boolean {
    const next: Record<string, string> = {};
    if (!concept.trim()) next.concept = 'Escribe qué estás cobrando.';
    if (amountCents === null) next.amount = 'Ingresa un monto mayor a $0.00.';
    if (!dueDate) next.dueDate = 'Elige una fecha límite.';
    else if (dueDate < todayInMexico()) next.dueDate = 'La fecha límite no puede ser anterior a hoy.';
    setErrors(next);
    return Object.keys(next).length === 0;
  }

  async function handleCreate() {
    if (!customer || amountCents === null) return;
    setBusy(true);
    setErrors({});
    try {
      const result = await onCreateCharge({ customerId: customer.customerId, concept: concept.trim(), amountCents, dueDate, method });
      onCreated(result);
    } catch (error) {
      setErrors({ form: error instanceof Error ? error.message : 'No fue posible crear el cobro.' });
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="db-card db-stepper" aria-labelledby={`${baseId}-title`}>
      <ol className="db-stepper-progress" aria-label="Pasos para crear tu primer cobro">
        {STEPS.map((item, index) => (
          <li
            key={item.id}
            className={index < stepIndex ? 'is-done' : index === stepIndex ? 'is-current' : undefined}
            aria-current={index === stepIndex ? 'step' : undefined}
          >
            <span className="db-stepper-dot">{index < stepIndex ? <CheckIcon /> : index + 1}</span>
            <span className="db-stepper-label">{item.label}</span>
          </li>
        ))}
      </ol>

      {step === 'welcome' && (
        <div className="db-stack">
          <span className="db-stepper-eyebrow">Tu cuenta está lista</span>
          <h2 id={`${baseId}-title`}>¡Hola, {profile.name}! 👋</h2>
          <p className="db-muted">
            <strong>{profile.business.name}</strong> ya puede cobrar. Vamos a crear tu primer cobro en menos de un minuto.
          </p>
          <ul className="db-stepper-benefits">
            <li>Tu cliente paga con un enlace, un QR CoDi o un cobro directo a su teléfono.</li>
            <li>El dinero llega directo a tu CLABE {clabeLabel}; nosotros nunca lo tocamos.</li>
            <li>Ves en un solo lugar quién te pagó y quién no.</li>
          </ul>
          <div className="db-row-actions">
            <button type="button" className="db-btn-accent" onClick={() => goTo('customer')}>
              <BoltIcon />
              <span>Crear mi primer cobro</span>
            </button>
          </div>
        </div>
      )}

      {step === 'customer' && (
        <div className="db-stack">
          <h2 id={`${baseId}-title`}>¿A quién le vas a cobrar?</h2>
          {customers.length === 0 ? (
            <p className="db-muted">Todavía no tienes clientes. Registra uno para continuar.</p>
          ) : (
            <div className="db-stepper-options" role="group" aria-label="Cliente">
              {customers.map((row) => {
                const phone = customerPhones[row.customerId];
                const selected = row.customerId === customer?.customerId;
                return (
                  <button
                    key={row.customerId}
                    type="button"
                    className="db-method-card"
                    aria-pressed={selected}
                    onClick={() => setCustomerId(row.customerId)}
                  >
                    <span>
                      <span className="db-method-title">{row.name}</span>
                      <span className="db-method-desc">
                        {phone ? `+52 ${formatMxPhone(phone)}` : 'Sin teléfono'}
                        {row.name === profile.firstCustomer.name ? ' · Lo registraste en tu alta' : ''}
                      </span>
                    </span>
                  </button>
                );
              })}
            </div>
          )}
          <div className="db-row-actions">
            <button type="button" className="db-btn-secondary" onClick={() => goTo('welcome')}>
              Atrás
            </button>
            <button type="button" className="db-btn-ghost" onClick={onRegisterCustomer}>
              <PlusIcon />
              <span>Registrar otro cliente</span>
            </button>
            <button type="button" className="db-btn-accent" disabled={!customer} onClick={() => goTo('charge')}>
              <span>Continuar</span>
              <ArrowRightIcon />
            </button>
          </div>
        </div>
      )}

      {step === 'charge' && (
        <div className="db-stack">
          <h2 id={`${baseId}-title`}>¿Qué le vas a cobrar a {customer?.name}?</h2>

          <div className="db-stack" style={{ gap: '8px' }}>
            <div className="db-chips" role="group" aria-label="Conceptos sugeridos">
              {QUICK_CONCEPTS.map((preset) => (
                <button key={preset} type="button" className="db-chip" aria-pressed={concept === preset} onClick={() => setConcept(preset)}>
                  {preset}
                </button>
              ))}
            </div>
            <div className="form-field">
              <label htmlFor={`${baseId}-concept`}>Concepto</label>
              <input
                id={`${baseId}-concept`}
                maxLength={CONCEPT_MAX_LENGTH}
                value={concept}
                onChange={(event) => setConcept(event.target.value)}
                aria-invalid={Boolean(errors.concept)}
              />
              {errors.concept && <span className="field-error">{errors.concept}</span>}
            </div>
          </div>

          <div className="db-stack" style={{ gap: '8px' }}>
            <div className="db-chips" role="group" aria-label="Montos sugeridos en pesos">
              {QUICK_AMOUNT_PESOS.map((preset) => (
                <button key={preset} type="button" className="db-chip" aria-pressed={Number(amount) === preset} onClick={() => setAmount(String(preset))}>
                  ${preset.toLocaleString('es-MX')}
                </button>
              ))}
            </div>
            <div className="db-form-grid">
              <div className="form-field">
                <label htmlFor={`${baseId}-amount`}>Monto en MXN</label>
                <input
                  id={`${baseId}-amount`}
                  type="number"
                  min="1"
                  step="0.01"
                  inputMode="decimal"
                  value={amount}
                  onChange={(event) => setAmount(event.target.value)}
                  aria-invalid={Boolean(errors.amount)}
                />
                {errors.amount && <span className="field-error">{errors.amount}</span>}
              </div>
              <div className="form-field">
                <label htmlFor={`${baseId}-due`}>Fecha límite de pago</label>
                <input
                  id={`${baseId}-due`}
                  type="date"
                  min={todayInMexico()}
                  value={dueDate}
                  onChange={(event) => setDueDate(event.target.value)}
                  aria-invalid={Boolean(errors.dueDate)}
                />
                {errors.dueDate && <span className="field-error">{errors.dueDate}</span>}
              </div>
            </div>
          </div>

          <div className="db-stack" style={{ gap: '8px' }}>
            <span style={{ fontSize: '0.85rem', fontWeight: 600 }}>¿Cómo te va a pagar?</span>
            <div className="db-method-grid" role="group" aria-label="Modalidad de cobro">
              {QUICK_PAYMENT_METHODS.map((item) => (
                <button key={item.id} type="button" className="db-method-card" aria-pressed={method === item.id} onClick={() => setMethod(item.id)}>
                  <span style={{ marginTop: '2px' }}>{item.id === 'CODI_QR' ? <QrCodeIcon /> : item.id === 'CODI_PUSH' ? <BoltIcon /> : <LinkIcon />}</span>
                  <span>
                    <span className="db-method-title">{item.title}</span>
                    <span className="db-method-desc">{item.description}</span>
                  </span>
                </button>
              ))}
            </div>
          </div>

          <div className="db-row-actions">
            <button type="button" className="db-btn-secondary" onClick={() => goTo('customer')}>
              Atrás
            </button>
            <button type="button" className="db-btn-accent" onClick={() => validateCharge() && goTo('confirm')}>
              <span>Revisar cobro</span>
              <ArrowRightIcon />
            </button>
          </div>
        </div>
      )}

      {step === 'confirm' && customer && amountCents !== null && (
        <div className="db-stack">
          <h2 id={`${baseId}-title`}>Revisa y crea tu cobro</h2>
          <div className="db-preview-card">
            <div className="db-preview-meta">
              <span>{profile.business.name}</span>
              <span>CLABE {clabeLabel}</span>
            </div>
            <strong>
              {concept.trim()} · {customer.name}
            </strong>
            <div className="db-preview-amount">{formatMxnCents(amountCents)}</div>
            <div className="db-preview-meta">
              <span>Vence {formatDate(dueDate)}</span>
              <span>{methodInfo?.title ?? method}</span>
            </div>
          </div>
          {errors.form && (
            <p role="alert" className="db-error-banner">
              {errors.form}
            </p>
          )}
          <div className="db-row-actions">
            <button type="button" className="db-btn-secondary" disabled={busy} onClick={() => goTo('charge')}>
              Atrás
            </button>
            <button type="button" className="db-btn-accent" disabled={busy} onClick={() => void handleCreate()}>
              <CheckIcon />
              <span>{busy ? 'Creando cobro…' : 'Crear cobro'}</span>
            </button>
          </div>
        </div>
      )}
    </section>
  );
}
