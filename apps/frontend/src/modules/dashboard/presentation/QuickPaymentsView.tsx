import { useId, useState, type FormEvent } from 'react';
import type { CustomerRowDto, DashboardProfileDto, PaymentMethodDto } from '@platform/contracts';
import type { CreatedChargeResult } from '../application/dashboard-gateway';
import {
  CONCEPT_MAX_LENGTH,
  QUICK_AMOUNT_PESOS,
  QUICK_CONCEPTS,
  QUICK_PAYMENT_METHODS,
  customerDisplayLabel,
  formatDate,
  formatMxnCents,
  pesosToCents,
  todayInMexico,
  type CustomerProfileRecord,
} from '../domain/dashboard';
import { ChargeDeliveryPanels } from './ChargeDeliveryPanels';
import { ArrowRightIcon, BoltIcon, CheckIcon, LinkIcon, PlusIcon, QrCodeIcon } from './DashboardIcons';

interface QuickPaymentsViewProps {
  readonly profile: DashboardProfileDto;
  readonly customers: readonly CustomerRowDto[];
  readonly customerDetails: Readonly<Record<string, CustomerProfileRecord>>;
  readonly selectedCustomerId: string;
  readonly clabeLabel: string;
  readonly onSelectCustomer: (customerId: string) => void;
  readonly onCreateCharge: (input: {
    customerId: string;
    concept: string;
    amountCents: number;
    dueDate: string;
    method: PaymentMethodDto;
  }) => Promise<CreatedChargeResult>;
  readonly onGoToClients: () => void;
  readonly onGoToCharges: () => void;
  readonly onNotify: (message: string) => void;
}

export function QuickPaymentsView({
  profile,
  customers,
  customerDetails,
  selectedCustomerId,
  clabeLabel,
  onSelectCustomer,
  onCreateCharge,
  onGoToClients,
  onGoToCharges,
  onNotify,
}: QuickPaymentsViewProps) {
  const [concept, setConcept] = useState('Mensualidad');
  const [amount, setAmount] = useState('500');
  const [dueDate, setDueDate] = useState(todayInMexico());
  const [method, setMethod] = useState<PaymentMethodDto>('CODI_QR');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [createdResult, setCreatedResult] = useState<CreatedChargeResult | null>(null);
  const baseId = useId();

  const activeCustomer = customers.find((row) => row.customerId === selectedCustomerId) ?? customers[0];
  const effectiveCustomerId = activeCustomer?.customerId ?? '';
  const activeDetail = effectiveCustomerId ? customerDetails[effectiveCustomerId] : undefined;
  const activePhone = activeDetail?.phone;
  const previewAmountCents = pesosToCents(amount) ?? 0;

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const amountCents = pesosToCents(amount);
    if (!effectiveCustomerId || amountCents === null || !concept.trim()) {
      setError('Selecciona un cliente, escribe un concepto y un monto mayor a $0.00.');
      return;
    }
    setError(null);
    setBusy(true);
    try {
      const result = await onCreateCharge({
        customerId: effectiveCustomerId,
        concept: concept.trim(),
        amountCents,
        dueDate,
        method,
      });
      setCreatedResult(result);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'No fue posible crear el cobro.');
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="db-stack">
      <div className="db-section-header">
        <div>
          <h2>Pagos rápidos</h2>
          <p className="db-muted">Genera un enlace de pago o QR CoDi en segundos.</p>
        </div>
        <button type="button" className="db-btn-secondary" onClick={onGoToClients}>
          <PlusIcon />
          <span>Nuevo cliente</span>
        </button>
      </div>

      <div className="db-payment-grid">
        <form className="db-card db-stack" onSubmit={handleSubmit} noValidate>
          <div className="form-field">
            <label htmlFor={`${baseId}-customer`}>Cliente</label>
            <select
              id={`${baseId}-customer`}
              value={effectiveCustomerId}
              onChange={(event) => {
                onSelectCustomer(event.target.value);
                setCreatedResult(null);
              }}
              disabled={busy || customers.length === 0}
            >
              {customers.map((row) => {
                const detail = customerDetails[row.customerId];
                return (
                  <option key={row.customerId} value={row.customerId}>
                    {detail ? customerDisplayLabel(detail) : row.name}
                  </option>
                );
              })}
            </select>
          </div>

          <div className="db-stack" style={{ gap: '8px' }}>
            <div className="db-chips" role="group" aria-label="Conceptos rápidos">
              {QUICK_CONCEPTS.map((preset) => (
                <button
                  key={preset}
                  type="button"
                  className="db-chip"
                  aria-pressed={concept === preset}
                  onClick={() => setConcept(preset)}
                >
                  {preset}
                </button>
              ))}
            </div>
            <div className="form-field">
              <label htmlFor={`${baseId}-concept`}>Concepto (máx. {CONCEPT_MAX_LENGTH} caracteres)</label>
              <input
                id={`${baseId}-concept`}
                maxLength={CONCEPT_MAX_LENGTH}
                value={concept}
                onChange={(event) => setConcept(event.target.value)}
                required
              />
            </div>
          </div>

          <div className="db-stack" style={{ gap: '8px' }}>
            <div className="db-chips" role="group" aria-label="Montos sugeridos en pesos">
              {QUICK_AMOUNT_PESOS.map((preset) => (
                <button
                  key={preset}
                  type="button"
                  className="db-chip"
                  aria-pressed={Number(amount) === preset}
                  onClick={() => setAmount(String(preset))}
                >
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
                  min="0.01"
                  step="0.01"
                  inputMode="decimal"
                  value={amount}
                  onChange={(event) => setAmount(event.target.value)}
                  required
                />
              </div>
              <div className="form-field">
                <label htmlFor={`${baseId}-due`}>Fecha límite</label>
                <input
                  id={`${baseId}-due`}
                  type="date"
                  value={dueDate}
                  onChange={(event) => setDueDate(event.target.value)}
                  required
                />
              </div>
            </div>
          </div>

          <div className="db-stack" style={{ gap: '8px' }}>
            <span style={{ fontSize: '0.85rem', fontWeight: 600 }}>Modalidad de cobro</span>
            <div className="db-method-grid" role="group" aria-label="Modalidad de cobro">
              {QUICK_PAYMENT_METHODS.map((item) => (
                <button
                  key={item.id}
                  type="button"
                  className="db-method-card"
                  aria-pressed={method === item.id}
                  onClick={() => setMethod(item.id)}
                >
                  <span style={{ marginTop: '2px' }}>
                    {item.id === 'CODI_QR' ? <QrCodeIcon /> : item.id === 'CODI_PUSH' ? <BoltIcon /> : <LinkIcon />}
                  </span>
                  <span>
                    <span className="db-method-title">{item.title}</span>
                    <span className="db-method-desc">{item.description}</span>
                  </span>
                </button>
              ))}
            </div>
          </div>

          {error && (
            <p role="alert" className="db-error-banner">
              {error}
            </p>
          )}

          <button type="submit" className="db-btn-accent" disabled={busy || !effectiveCustomerId}>
            <span>
              {busy
                ? 'Generando cobro...'
                : method === 'CODI_QR'
                  ? 'Generar QR CoDi'
                  : method === 'CODI_PUSH'
                    ? 'Enviar cobro CoDi'
                    : 'Generar enlace de pago'}
            </span>
            <ArrowRightIcon />
          </button>
        </form>

        <section className="db-card db-stack" aria-label="Vista previa y envío">
          <div className="db-section-header">
            <h3>{createdResult ? 'Cobro listo para compartir' : 'Vista previa del cobro'}</h3>
            {createdResult && (
              <span className="db-badge is-pending">
                <CheckIcon />
                <span>Pendiente de pago</span>
              </span>
            )}
          </div>

          <div className="db-preview-card">
            <div className="db-preview-meta">
              <span>{profile.business.name.trim() || profile.name}</span>
              <span>CLABE {clabeLabel}</span>
            </div>
            <strong>
              {(createdResult ? createdResult.charge.concept : concept) || 'Concepto'} ·{' '}
              {createdResult ? createdResult.charge.customerName : activeCustomer?.name ?? 'Cliente'}
            </strong>
            <div className="db-preview-amount">
              {formatMxnCents(createdResult ? createdResult.charge.amountCents : previewAmountCents)}
            </div>
            <div className="db-preview-meta">
              <span>Vence {formatDate(createdResult ? createdResult.charge.dueDate : dueDate)}</span>
              <span>
                {(createdResult?.method ?? method) === 'CODI_QR'
                  ? 'CoDi 0% comisión'
                  : (createdResult?.method ?? method) === 'CODI_PUSH'
                    ? 'CoDi al teléfono del cliente'
                    : 'Enlace de pago'}
              </span>
            </div>
          </div>

          <ChargeDeliveryPanels
            createdResult={createdResult}
            method={method}
            clabeLabel={clabeLabel}
            fallbackCustomerName={activeCustomer?.name ?? ''}
            previewAmountCents={previewAmountCents}
            concept={concept}
            customerPhone={activePhone}
            onNotify={onNotify}
            extraActions={
              <button type="button" className="db-btn-ghost" onClick={onGoToCharges}>
                <span>Ver lista de pagos</span>
                <ArrowRightIcon />
              </button>
            }
          />
        </section>
      </div>
    </div>
  );
}
