import { useId, useState, type FormEvent } from 'react';
import type { CustomerRowDto, DashboardCustomersDto, DashboardSummaryDto, VendorClientDto } from '@platform/contracts';
import {
  CUSTOMER_ACTIVITIES,
  CUSTOMER_ENTITY_LABELS,
  CUSTOMER_TAX_REGIMES,
  PAYMENT_METHOD_LABELS,
  STANDING_LABELS,
  formatDate,
  formatMxPhone,
  formatMxnCents,
  mxPhoneDigits,
  normalizeNewCustomerInput,
  rfcKindLabel,
  validateNewCustomer,
  type CustomerEntityType,
  type CustomerFieldErrors,
  type CustomerFieldName,
  type CustomerProfileRecord,
  type NewCustomerInput,
} from '../domain/dashboard';
import { BoltIcon, BuildingIcon, EditIcon, ListIcon, PlusIcon, UserIcon, XIcon } from './DashboardIcons';

interface CustomersViewProps {
  readonly customersData: DashboardCustomersDto;
  readonly summary: DashboardSummaryDto;
  readonly customerDetails: Readonly<Record<string, CustomerProfileRecord>>;
  readonly onCreateCustomer: (input: NewCustomerInput) => Promise<VendorClientDto>;
  readonly onUpdateCustomer: (customerId: string, input: NewCustomerInput) => Promise<VendorClientDto>;
  readonly onChargeCustomer: (customerId: string) => void;
  readonly onViewCustomerCharges: (customerName: string) => void;
}

type EntityFilter = 'all' | CustomerEntityType;

const STANDING_BADGE_CLASS: Record<CustomerRowDto['standing'], string> = {
  overdue: 'is-overdue',
  pending: 'is-pending',
  up_to_date: 'is-paid',
};

function createEmptyForm(entityType: CustomerEntityType = 'individual'): NewCustomerInput {
  return {
    entityType,
    name: '',
    phone: '',
    email: '',
    business: {
      legalName: '',
      tradeName: '',
      rfc: '',
      activity: '',
      taxRegime: '',
      fiscalAddress: '',
    },
  };
}

function recordToForm(record: CustomerProfileRecord): NewCustomerInput {
  return {
    entityType: record.entityType,
    name: record.name,
    phone: record.phone,
    email: record.email,
    business: {
      legalName: record.business?.legalName ?? '',
      tradeName: record.business?.tradeName ?? '',
      rfc: record.business?.rfc ?? '',
      activity: record.business?.activity ?? '',
      taxRegime: record.business?.taxRegime ?? '',
      fiscalAddress: record.business?.fiscalAddress ?? '',
    },
  };
}

export function CustomersView({
  customersData,
  summary,
  customerDetails,
  onCreateCustomer,
  onUpdateCustomer,
  onChargeCustomer,
  onViewCustomerCharges,
}: CustomersViewProps) {
  const [search, setSearch] = useState('');
  const [entityFilter, setEntityFilter] = useState<EntityFilter>('all');
  const [formMode, setFormMode] = useState<{ open: boolean; editingCustomerId: string | null }>({
    open: false,
    editingCustomerId: null,
  });
  const [form, setForm] = useState<NewCustomerInput>(() => createEmptyForm('individual'));
  const [showOptionalIndividualTax, setShowOptionalIndividualTax] = useState(false);
  const [errors, setErrors] = useState<CustomerFieldErrors>({});
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const baseId = useId();

  const normalizedTerm = search.trim().toLowerCase();
  const visibleCustomers = customersData.customers.filter((row) => {
    const details = customerDetails[row.customerId];
    const entityType: CustomerEntityType = details?.entityType ?? 'individual';
    if (entityFilter !== 'all' && entityType !== entityFilter) return false;
    if (!normalizedTerm) return true;

    const haystacks = [
      row.name,
      details?.name,
      details?.phone,
      details?.email,
      details?.business?.legalName,
      details?.business?.tradeName,
      details?.business?.rfc,
      details?.business?.activity,
      details?.business?.taxRegime,
      details?.business?.fiscalAddress,
    ];
    return haystacks.some((value) => value && value.toLowerCase().includes(normalizedTerm));
  });

  function clearError(field: CustomerFieldName) {
    setErrors((prev) => ({ ...prev, [field]: undefined }));
  }

  function updateBusinessField(field: 'legalName' | 'tradeName' | 'rfc' | 'activity' | 'taxRegime' | 'fiscalAddress', value: string) {
    setForm((prev) => ({
      ...prev,
      business: {
        legalName: prev.business?.legalName ?? '',
        ...prev.business,
        [field]: field === 'rfc' ? value.toUpperCase() : value,
      },
    }));
    clearError(field);
  }

  function openNewCustomerForm() {
    setForm(createEmptyForm('individual'));
    setShowOptionalIndividualTax(false);
    setErrors({});
    setSubmitError(null);
    setFormMode({ open: true, editingCustomerId: null });
  }

  function openEditCustomerForm(row: CustomerRowDto) {
    const details = customerDetails[row.customerId];
    const nextForm = details
      ? recordToForm(details)
      : { ...createEmptyForm('individual'), name: row.name };
    setForm(nextForm);
    setShowOptionalIndividualTax(
      nextForm.entityType === 'individual' &&
        Boolean(
          nextForm.business?.legalName ||
            nextForm.business?.tradeName ||
            nextForm.business?.rfc ||
            nextForm.business?.activity ||
            nextForm.business?.taxRegime ||
            nextForm.business?.fiscalAddress,
        ),
    );
    setErrors({});
    setSubmitError(null);
    setFormMode({ open: true, editingCustomerId: row.customerId });
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const cleaned = normalizeNewCustomerInput(form);
    const nextErrors = validateNewCustomer(cleaned);
    setErrors(nextErrors);
    if (Object.keys(nextErrors).length) return;

    setSaving(true);
    setSubmitError(null);
    try {
      if (formMode.editingCustomerId) {
        await onUpdateCustomer(formMode.editingCustomerId, cleaned);
      } else {
        await onCreateCustomer(cleaned);
      }
      setForm(createEmptyForm('individual'));
      setFormMode({ open: false, editingCustomerId: null });
    } catch (err) {
      setSubmitError(err instanceof Error ? err.message : 'No fue posible guardar el cliente.');
    } finally {
      setSaving(false);
    }
  }

  const isCompany = form.entityType === 'company';
  const showBusinessBlock = isCompany || showOptionalIndividualTax;

  const companyCount = customersData.customers.filter(
    (row) => (customerDetails[row.customerId]?.entityType ?? 'individual') === 'company',
  ).length;
  const individualCount = customersData.customers.length - companyCount;

  return (
    <div className="db-stack">
      <div className="db-section-header">
        <div>
          <h2>Clientes y saldos</h2>
          <p className="db-muted">
            {customersData.counts.overdue} vencidos · {customersData.counts.pending} pendientes · {customersData.counts.upToDate} al día · plan{' '}
            {summary.customers.plan} ({summary.customers.active}/{summary.customers.limit ?? '∞'})
          </p>
        </div>
        <button
          type="button"
          className="db-btn-primary"
          onClick={() => (formMode.open ? setFormMode({ open: false, editingCustomerId: null }) : openNewCustomerForm())}
        >
          {formMode.open ? <XIcon /> : <PlusIcon />}
          <span>{formMode.open ? 'Cerrar formulario' : 'Registrar cliente'}</span>
        </button>
      </div>

      {formMode.open && (
        <form
          className="db-card db-stack"
          onSubmit={handleSubmit}
          noValidate
          aria-label={formMode.editingCustomerId ? 'Editar datos del cliente' : 'Registrar nuevo cliente'}
        >
          <div>
            <h3>{formMode.editingCustomerId ? 'Editar ficha del cliente' : 'Nuevo cliente'}</h3>
            <p className="db-muted">
              Selecciona si es persona física o persona moral / comercio para capturar los datos necesarios de cobro y facturación.
            </p>
          </div>

          <div className="db-stack" style={{ gap: '8px' }}>
            <span style={{ fontSize: '0.85rem', fontWeight: 600 }}>Tipo de cliente</span>
            <div className="db-segmented" role="group" aria-label="Tipo de cliente">
              <button
                type="button"
                className="db-segmented-btn"
                aria-pressed={form.entityType === 'individual'}
                onClick={() => {
                  setForm((prev) => ({ ...prev, entityType: 'individual' }));
                  clearError('legalName');
                }}
              >
                <UserIcon />
                <span>Persona física</span>
              </button>
              <button
                type="button"
                className="db-segmented-btn"
                aria-pressed={form.entityType === 'company'}
                onClick={() => setForm((prev) => ({ ...prev, entityType: 'company' }))}
              >
                <BuildingIcon />
                <span>Persona moral / Comercio</span>
              </button>
            </div>
          </div>

          <div className="db-form-grid">
            <div className="form-field">
              <label htmlFor={`${baseId}-cust-name`}>
                {isCompany ? 'Contacto principal / encargado de pagos *' : 'Nombre completo *'}
              </label>
              <input
                id={`${baseId}-cust-name`}
                value={form.name}
                placeholder={isCompany ? 'Ej. Sofía Ramírez' : 'Ej. Laura Mendoza'}
                aria-invalid={Boolean(errors.name)}
                aria-describedby={errors.name ? `${baseId}-cust-name-err` : undefined}
                onChange={(event) => {
                  setForm((prev) => ({ ...prev, name: event.target.value }));
                  clearError('name');
                }}
                required
              />
              {errors.name && (
                <small id={`${baseId}-cust-name-err`} role="alert" className="field-error">
                  {errors.name}
                </small>
              )}
            </div>

            <div className="form-field">
              <label htmlFor={`${baseId}-cust-phone`}>Teléfono (10 dígitos) *</label>
              <div className="masked-input">
                <span className="masked-input-prefix" aria-hidden="true">
                  +52
                </span>
                <input
                  id={`${baseId}-cust-phone`}
                  type="tel"
                  inputMode="tel"
                  placeholder="55 1234 5678"
                  value={formatMxPhone(form.phone)}
                  aria-invalid={Boolean(errors.phone)}
                  aria-describedby={errors.phone ? `${baseId}-cust-phone-err` : undefined}
                  onChange={(event) => {
                    setForm((prev) => ({ ...prev, phone: mxPhoneDigits(event.target.value) }));
                    clearError('phone');
                  }}
                  required
                />
              </div>
              {errors.phone && (
                <small id={`${baseId}-cust-phone-err`} role="alert" className="field-error">
                  {errors.phone}
                </small>
              )}
            </div>

            <div className="form-field db-field-span-2">
              <label htmlFor={`${baseId}-cust-email`}>
                {isCompany ? 'Correo de pagos o facturación *' : 'Correo electrónico *'}
              </label>
              <input
                id={`${baseId}-cust-email`}
                type="email"
                inputMode="email"
                placeholder={isCompany ? 'Ej. pagos@empresa.mx' : 'Ej. laura@ejemplo.mx'}
                value={form.email}
                aria-invalid={Boolean(errors.email)}
                aria-describedby={errors.email ? `${baseId}-cust-email-err` : undefined}
                onChange={(event) => {
                  setForm((prev) => ({ ...prev, email: event.target.value }));
                  clearError('email');
                }}
                required
              />
              {errors.email && (
                <small id={`${baseId}-cust-email-err`} role="alert" className="field-error">
                  {errors.email}
                </small>
              )}
            </div>
          </div>

          {!isCompany && (
            <div>
              <button
                type="button"
                className="db-btn-ghost"
                aria-expanded={showOptionalIndividualTax}
                onClick={() => setShowOptionalIndividualTax((prev) => !prev)}
              >
                {showOptionalIndividualTax
                  ? 'Ocultar datos fiscales o negocio'
                  : '+ Agregar RFC o nombre comercial (opcional)'}
              </button>
            </div>
          )}

          {showBusinessBlock && (
            <div className="db-company-box" role="group" aria-label="Detalles del negocio o empresa">
              <p className="db-company-box-title">
                {isCompany ? 'Detalles del negocio o empresa' : 'Datos fiscales / negocio propio (opcional)'}
              </p>

              <div className="db-form-grid">
                <div className="form-field">
                  <label htmlFor={`${baseId}-biz-legal`}>
                    {isCompany ? 'Razón social o nombre del comercio *' : 'Nombre comercial o negocio'}
                  </label>
                  <input
                    id={`${baseId}-biz-legal`}
                    value={form.business?.legalName ?? ''}
                    placeholder={isCompany ? 'Ej. Comercializadora del Norte S.A. de C.V.' : 'Ej. Consultorio Mendoza'}
                    aria-invalid={Boolean(errors.legalName)}
                    aria-describedby={errors.legalName ? `${baseId}-biz-legal-err` : undefined}
                    onChange={(event) => updateBusinessField('legalName', event.target.value)}
                    required={isCompany}
                  />
                  {errors.legalName && (
                    <small id={`${baseId}-biz-legal-err`} role="alert" className="field-error">
                      {errors.legalName}
                    </small>
                  )}
                </div>

                <div className="form-field">
                  <label htmlFor={`${baseId}-biz-trade`}>Nombre comercial (opcional)</label>
                  <input
                    id={`${baseId}-biz-trade`}
                    value={form.business?.tradeName ?? ''}
                    placeholder="Ej. Norte Industrial"
                    aria-invalid={Boolean(errors.tradeName)}
                    onChange={(event) => updateBusinessField('tradeName', event.target.value)}
                  />
                  {errors.tradeName && (
                    <small role="alert" className="field-error">
                      {errors.tradeName}
                    </small>
                  )}
                </div>

                <div className="form-field">
                  <label htmlFor={`${baseId}-biz-rfc`}>
                    RFC {isCompany ? '(12 moral / 13 comercio)' : '(13 caracteres)'}
                  </label>
                  <input
                    id={`${baseId}-biz-rfc`}
                    maxLength={13}
                    value={form.business?.rfc ?? ''}
                    placeholder={isCompany ? 'Ej. CNO180412AB1' : 'Ej. MELJ900101XXX'}
                    aria-invalid={Boolean(errors.rfc)}
                    aria-describedby={errors.rfc ? `${baseId}-biz-rfc-err` : undefined}
                    onChange={(event) => updateBusinessField('rfc', event.target.value)}
                  />
                  {errors.rfc && (
                    <small id={`${baseId}-biz-rfc-err`} role="alert" className="field-error">
                      {errors.rfc}
                    </small>
                  )}
                </div>

                <div className="form-field">
                  <label htmlFor={`${baseId}-biz-activity`}>Giro del negocio</label>
                  <select
                    id={`${baseId}-biz-activity`}
                    value={form.business?.activity ?? ''}
                    onChange={(event) => updateBusinessField('activity', event.target.value)}
                  >
                    <option value="">Sin especificar</option>
                    {CUSTOMER_ACTIVITIES.map((option) => (
                      <option key={option} value={option}>
                        {option}
                      </option>
                    ))}
                  </select>
                </div>

                <div className="form-field">
                  <label htmlFor={`${baseId}-biz-regime`}>Régimen fiscal</label>
                  <select
                    id={`${baseId}-biz-regime`}
                    value={form.business?.taxRegime ?? ''}
                    onChange={(event) => updateBusinessField('taxRegime', event.target.value)}
                  >
                    <option value="">Sin especificar</option>
                    {CUSTOMER_TAX_REGIMES.map((option) => (
                      <option key={option} value={option}>
                        {option}
                      </option>
                    ))}
                  </select>
                </div>

                <div className="form-field">
                  <label htmlFor={`${baseId}-biz-address`}>Dirección fiscal o sucursal (opcional)</label>
                  <input
                    id={`${baseId}-biz-address`}
                    maxLength={255}
                    value={form.business?.fiscalAddress ?? ''}
                    placeholder="Ej. Av. Reforma 450, Col. Juárez, CDMX"
                    aria-invalid={Boolean(errors.fiscalAddress)}
                    onChange={(event) => updateBusinessField('fiscalAddress', event.target.value)}
                  />
                  {errors.fiscalAddress && (
                    <small role="alert" className="field-error">
                      {errors.fiscalAddress}
                    </small>
                  )}
                </div>
              </div>
            </div>
          )}

          {submitError && (
            <p role="alert" className="db-error-banner">
              {submitError}
            </p>
          )}

          <div className="db-row-actions">
            <button type="submit" className="db-btn-accent" disabled={saving}>
              {saving ? 'Guardando...' : formMode.editingCustomerId ? 'Guardar cambios' : 'Guardar cliente'}
            </button>
            <button
              type="button"
              className="db-btn-secondary"
              onClick={() => setFormMode({ open: false, editingCustomerId: null })}
            >
              Cancelar
            </button>
          </div>
        </form>
      )}

      <div className="db-section-header">
        <div className="db-filter-row" role="tablist" aria-label="Filtrar por tipo de cliente">
          <button
            type="button"
            role="tab"
            aria-selected={entityFilter === 'all'}
            className="db-filter-pill"
            onClick={() => setEntityFilter('all')}
          >
            Todos ({customersData.customers.length})
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={entityFilter === 'company'}
            className="db-filter-pill"
            onClick={() => setEntityFilter('company')}
          >
            Empresas / Comercios ({companyCount})
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={entityFilter === 'individual'}
            className="db-filter-pill"
            onClick={() => setEntityFilter('individual')}
          >
            Personas físicas ({individualCount})
          </button>
        </div>

        <input
          type="search"
          className="db-search-input"
          placeholder="Buscar por nombre, empresa, RFC, teléfono..."
          aria-label="Buscar clientes"
          value={search}
          onChange={(event) => setSearch(event.target.value)}
        />
      </div>

      {visibleCustomers.length === 0 ? (
        <div className="db-card">
          <p className="db-muted">No hay clientes que coincidan con este filtro o búsqueda.</p>
        </div>
      ) : (
        <div className="db-clients-grid">
          {visibleCustomers.map((row) => {
            const details = customerDetails[row.customerId];
            const entityType: CustomerEntityType = details?.entityType ?? 'individual';
            const biz = details?.business;
            const phone = details?.phone ?? '';
            const email = details?.email ?? '';
            const rfcTag = rfcKindLabel(biz?.rfc);
            const hasBusinessData = Boolean(
              biz?.legalName || biz?.tradeName || biz?.rfc || biz?.activity || biz?.taxRegime || biz?.fiscalAddress,
            );

            return (
              <article key={row.customerId} className="db-card db-client-card">
                <div className="db-customer-badges">
                  <span className={`db-badge ${entityType === 'company' ? 'is-company' : 'is-individual'}`}>
                    {entityType === 'company' ? <BuildingIcon /> : <UserIcon />}
                    <span>{CUSTOMER_ENTITY_LABELS[entityType]}</span>
                  </span>
                  <span className={`db-badge ${STANDING_BADGE_CLASS[row.standing]}`}>
                    {STANDING_LABELS[row.standing]}
                  </span>
                </div>

                <div>
                  <h3>{entityType === 'company' && biz?.legalName ? biz.legalName : details?.name ?? row.name}</h3>
                  {entityType === 'company' && biz?.tradeName && biz.tradeName !== biz.legalName && (
                    <p className="db-muted">Nombre comercial: {biz.tradeName}</p>
                  )}
                  {entityType === 'individual' && biz?.tradeName && (
                    <p className="db-muted">Negocio: {biz.tradeName}</p>
                  )}
                </div>

                {/* Full customer & business profile details */}
                <dl className="db-customer-details" aria-label={`Datos de ${row.name}`}>
                  {entityType === 'company' && (
                    <div className="db-customer-details-item">
                      <dt>Contacto de pagos</dt>
                      <dd>{details?.name ?? row.name}</dd>
                    </div>
                  )}

                  <div className="db-customer-details-item">
                    <dt>Teléfono</dt>
                    <dd className="tabular">{phone ? `+52 ${formatMxPhone(phone)}` : 'No registrado'}</dd>
                  </div>

                  <div className="db-customer-details-item">
                    <dt>Correo</dt>
                    <dd>{email || 'No registrado'}</dd>
                  </div>

                  {hasBusinessData && (
                    <>
                      {entityType === 'individual' && biz?.legalName && (
                        <div className="db-customer-details-item">
                          <dt>Negocio / Razón social</dt>
                          <dd>{biz.legalName}</dd>
                        </div>
                      )}
                      {biz?.rfc && (
                        <div className="db-customer-details-item">
                          <dt>RFC</dt>
                          <dd className="db-mono">
                            {biz.rfc}
                            {rfcTag ? ` · ${rfcTag}` : ''}
                          </dd>
                        </div>
                      )}
                      {biz?.activity && (
                        <div className="db-customer-details-item">
                          <dt>Giro</dt>
                          <dd>{biz.activity}</dd>
                        </div>
                      )}
                      {biz?.taxRegime && (
                        <div className="db-customer-details-item">
                          <dt>Régimen fiscal</dt>
                          <dd>{biz.taxRegime}</dd>
                        </div>
                      )}
                      {biz?.fiscalAddress && (
                        <div className="db-customer-details-item db-field-span-2">
                          <dt>Dirección fiscal / sucursal</dt>
                          <dd>{biz.fiscalAddress}</dd>
                        </div>
                      )}
                    </>
                  )}
                </dl>

                <div className="db-client-balances">
                  <div>
                    <span>Por cobrar</span>
                    <strong>{formatMxnCents(row.balance.outstandingCents)}</strong>
                  </div>
                  <div>
                    <span>Vencido</span>
                    <strong style={{ color: row.balance.overdueCents > 0 ? '#b45309' : undefined }}>
                      {formatMxnCents(row.balance.overdueCents)}
                    </strong>
                  </div>
                </div>

                <p className="db-muted">
                  {row.lastPayment
                    ? `Último pago: ${formatMxnCents(row.lastPayment.amountCents)} (${PAYMENT_METHOD_LABELS[row.lastPayment.method]}) · `
                    : 'Sin pagos previos · '}
                  {row.oldestDueDate
                    ? `Vencido desde ${formatDate(row.oldestDueDate)}`
                    : row.nextDueDate
                      ? `Próximo vencimiento ${formatDate(row.nextDueDate)}`
                      : 'Sin cobros pendientes'}
                </p>

                <div className="db-row-actions">
                  <button type="button" className="db-btn-accent" onClick={() => onChargeCustomer(row.customerId)}>
                    <BoltIcon />
                    <span>Cobrar</span>
                  </button>
                  <button type="button" className="db-btn-secondary" onClick={() => onViewCustomerCharges(row.name)}>
                    <ListIcon />
                    <span>Ver pagos</span>
                  </button>
                  <button type="button" className="db-btn-ghost" onClick={() => openEditCustomerForm(row)}>
                    <EditIcon />
                    <span>Editar datos</span>
                  </button>
                </div>
              </article>
            );
          })}
        </div>
      )}
    </div>
  );
}
