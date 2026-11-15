import { useCallback, useEffect, useMemo, useState, type FormEvent } from 'react';
import type {
  DashboardChargeDto,
  DashboardChargesDto,
  DashboardCustomersDto,
  DashboardProfileDto,
  DashboardSummaryDto,
  ManualPaymentMethod,
  UpdateOnboardingProfileRequest,
} from '@platform/contracts';
import {
  DashboardApiError,
  type CustomersDirectoryGateway,
  type DashboardGateway,
  type NewChargeInput,
  type PaymentsGateway,
} from '../application/dashboard-gateway';
import {
  CONCEPT_MAX_LENGTH,
  NOTE_MAX_LENGTH,
  chargesQueryFor,
  formatMxnCents,
  parseTags,
  pesosToCents,
  type ChargeFilter,
  type CustomerProfileRecord,
  type DashboardScreen,
  type NewCustomerInput,
} from '../domain/dashboard';
import { ChargeActionModal, type ActiveChargeModal } from './ChargeActionModal';
import { CustomersView } from './CustomersView';
import { BoltIcon, CheckIcon, HomeIcon, ListIcon, PlusIcon, UserIcon, UsersIcon } from './DashboardIcons';
import { DashboardOverview } from './DashboardOverview';
import { FirstChargeStepper } from './FirstChargeStepper';
import { PaymentsListView, type ChargeActionKind } from './PaymentsListView';
import { ProfileView, type DemoControls } from './ProfileView';
import { QuickPaymentsView } from './QuickPaymentsView';
import { clientToProfile, clientToRow } from '../infrastructure/http-clients-gateway';
import './dashboard.css';

interface DashboardWorkspaceProps {
  readonly gateway: DashboardGateway;
  readonly payments: PaymentsGateway;
  readonly customersDirectory: CustomersDirectoryGateway;
  readonly contractorId: string;
  readonly demo?: DemoControls;
  readonly onNotFound?: () => void;
  readonly onSignOut?: () => void;
  readonly onUpdateProfile?: (patch: UpdateOnboardingProfileRequest) => Promise<void>;
}

const NAV_TABS: readonly { readonly id: DashboardScreen; readonly label: string }[] = [
  { id: 'dashboard', label: 'Inicio' },
  { id: 'payment', label: 'Cobrar' },
  { id: 'charges', label: 'Pagos' },
  { id: 'clients', label: 'Clientes' },
];

export function DashboardWorkspace({
  gateway,
  payments,
  customersDirectory,
  contractorId,
  demo,
  onNotFound,
  onSignOut,
  onUpdateProfile,
}: DashboardWorkspaceProps) {
  const [screen, setScreen] = useState<DashboardScreen>('dashboard');
  const [filter, setFilter] = useState<ChargeFilter>('all');
  const [search, setSearch] = useState('');
  const [selectedCustomerId, setSelectedCustomerId] = useState('');

  const [profile, setProfile] = useState<DashboardProfileDto | null>(null);
  const [summary, setSummary] = useState<DashboardSummaryDto | null>(null);
  const [allCharges, setAllCharges] = useState<DashboardChargeDto[]>([]);
  const [charges, setCharges] = useState<DashboardChargesDto | null>(null);
  const [customers, setCustomers] = useState<DashboardCustomersDto | null>(null);
  const [customerDetails, setCustomerDetails] = useState<Record<string, CustomerProfileRecord>>({});
  // FirstChargeStepper only needs phones; derive them from the customer details.
  const customerPhones = useMemo(
    () => Object.fromEntries(Object.entries(customerDetails).map(([id, details]) => [id, details.phone])),
    [customerDetails],
  );
  const [loadError, setLoadError] = useState<string | null>(null);
  const [reloadKey, setReloadKey] = useState(0);

  const [modal, setModal] = useState<ActiveChargeModal | null>(null);
  const [modalErrors, setModalErrors] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState(false);
  const [toast, setToast] = useState<string | null>(null);
  const [highlightedChargeId, setHighlightedChargeId] = useState<string | null>(null);

  const reload = useCallback(() => setReloadKey((key) => key + 1), []);

  const notify = useCallback((message: string) => {
    setToast(message);
    window.setTimeout(() => setToast((current) => (current === message ? null : current)), 3600);
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    const { signal } = controller;

    async function loadWorkspace() {
      const nextProfile = await gateway.profile(contractorId, signal);
      const [nextSummary, everyCharge, filtered, nextClients] = await Promise.all([
        gateway.summary(contractorId, {}, signal),
        gateway.charges(contractorId, {}, signal),
        gateway.charges(contractorId, chargesQueryFor(filter, search), signal),
        customersDirectory.list(contractorId, signal),
      ]);
      if (signal.aborted) return;

      setProfile(nextProfile);
      setSummary(nextSummary);
      setAllCharges(everyCharge.charges);
      setCharges(filtered);
      setCustomers({
        customers: nextClients.map(clientToRow),
        counts: { all: nextClients.length, overdue: 0, pending: 0, upToDate: nextClients.length },
        nextCursor: null,
      });
      setCustomerDetails(Object.fromEntries(nextClients.map((client) => [client.clientId, clientToProfile(client)])));
      setLoadError(null);
      setSelectedCustomerId((current) => {
        if (current && nextClients.some((client) => client.clientId === current)) return current;
        return nextClients[0]?.clientId ?? '';
      });
    }

    loadWorkspace().catch((error: unknown) => {
      if (signal.aborted) return;
      if (
        error instanceof DashboardApiError &&
        (error.status === 401 || error.status === 403 || error.status === 404)
      ) {
        onNotFound?.();
        return;
      }
      setLoadError(error instanceof Error ? error.message : 'No fue posible cargar el tablero.');
    });

    return () => controller.abort();
  }, [gateway, customersDirectory, contractorId, filter, search, reloadKey, onNotFound]);

  function openActionModal(kind: ChargeActionKind, charge: DashboardChargeDto) {
    setModalErrors({});
    setModal({ kind, charge });
  }

  function handleModalError(error: unknown) {
    if (error instanceof DashboardApiError) {
      if (error.code === 'version_conflict') {
        reload();
        setModalErrors({ form: 'Este cobro cambió recientemente. Actualizamos el tablero; vuelve a abrirlo para guardar.' });
        return;
      }
      setModalErrors({ ...error.fields, ...(Object.keys(error.fields).length ? {} : { form: error.message }) });
      return;
    }
    setModalErrors({ form: 'No fue posible completar la acción.' });
  }

  async function runModalMutation(task: Promise<unknown>, successMessage: string) {
    setBusy(true);
    setModalErrors({});
    try {
      await task;
      setModal(null);
      notify(successMessage);
      reload();
    } catch (error) {
      handleModalError(error);
    } finally {
      setBusy(false);
    }
  }

  function handleSendReminder(charge: DashboardChargeDto, channel: 'WhatsApp' | 'Correo') {
    if (channel === 'WhatsApp') {
      const phone = customerDetails[charge.customerId]?.phone;
      if (phone) {
        const message = `Hola ${charge.customerName}, te recordamos tu cobro pendiente de ${formatMxnCents(charge.amountCents)} (${charge.concept}).`;
        window.open(`https://wa.me/52${phone}?text=${encodeURIComponent(message)}`, '_blank', 'noopener,noreferrer');
      }
    }
    void runModalMutation(payments.sendReminder(contractorId, charge.chargeId), `Recordatorio enviado por ${channel}.`);
  }

  function handleManualPayment(event: FormEvent<HTMLFormElement>, charge: DashboardChargeDto) {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    const method = String(data.get('method') ?? 'cash') as ManualPaymentMethod;
    const note = String(data.get('note') ?? '').trim();
    void runModalMutation(
      gateway.recordManualPayment(contractorId, charge.chargeId, {
        version: charge.version,
        method,
        ...(note ? { note } : {}),
      }),
      'Pago registrado correctamente.',
    );
  }

  function handleCancelCharge(event: FormEvent<HTMLFormElement>, charge: DashboardChargeDto) {
    event.preventDefault();
    const reason = String(new FormData(event.currentTarget).get('reason') ?? '').trim();
    void runModalMutation(
      gateway.cancelCharge(contractorId, charge.chargeId, {
        version: charge.version,
        ...(reason ? { reason } : {}),
      }),
      'Cobro cancelado.',
    );
  }

  function handleEditCharge(event: FormEvent<HTMLFormElement>, charge: DashboardChargeDto) {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    const body: { version: number; concept?: string; amountCents?: number; dueDate?: string; note?: string | null; tags?: string[] } = {
      version: charge.version,
      note: String(data.get('note') ?? '').trim().slice(0, NOTE_MAX_LENGTH) || null,
      tags: parseTags(String(data.get('tags') ?? '')),
    };
    if (charge.editableFields.includes('concept')) body.concept = String(data.get('concept') ?? '').trim().slice(0, CONCEPT_MAX_LENGTH);
    if (charge.editableFields.includes('dueDate')) body.dueDate = String(data.get('dueDate') ?? '');
    if (charge.editableFields.includes('amountCents')) {
      const amountCents = pesosToCents(String(data.get('amount') ?? ''));
      if (amountCents === null) {
        setModalErrors({ amountCents: 'Ingresa un monto válido.' });
        return;
      }
      body.amountCents = amountCents;
    }
    void runModalMutation(gateway.updateCharge(contractorId, charge.chargeId, body), 'Cambios guardados.');
  }

  async function handleCreateCharge(input: NewChargeInput) {
    const detail = customerDetails[input.customerId];
    if (!detail?.name.trim()) {
      throw new DashboardApiError('Este cliente no está registrado.', 422, undefined, {
        clientName: 'Este cliente no está registrado.',
      });
    }
    const result = await payments.createCharge(contractorId, {
      ...input,
      clientName: detail.name,
      clientPhone: detail.phone,
      clientEmail: detail.email,
    });
    notify(
      result.method === 'CODI_PUSH'
        ? `La solicitud de cobro se envió al teléfono ${detail.phone}.`
        : `Cobro generado: ${formatMxnCents(result.charge.amountCents)} para ${result.charge.customerName}.`,
    );
    reload();
    return result;
  }

  async function handleCreateCustomer(input: NewCustomerInput) {
    const created = await customersDirectory.createCustomer(contractorId, input);
    setSelectedCustomerId(created.clientId);
    notify(`Cliente registrado: ${created.name}.`);
    reload();
    return created;
  }

  async function handleUpdateCustomer(customerId: string, input: NewCustomerInput) {
    const updated = await customersDirectory.updateCustomer(contractorId, customerId, input);
    notify(`Cliente actualizado: ${updated.name}.`);
    reload();
    return updated;
  }

  if (loadError) {
    return (
      <section className="db-shell db-body">
        <p role="alert" className="db-error-banner">
          {loadError}
        </p>
        <button type="button" className="db-btn-primary" onClick={reload}>
          Reintentar
        </button>
      </section>
    );
  }

  if (!profile || !summary || !charges || !customers) {
    return (
      <section className="db-shell db-body" aria-busy="true">
        <p className="db-muted">Cargando tu espacio de cobros...</p>
      </section>
    );
  }

  const clabeLabel = profile.business.clabeLast4 ? `•••• ${profile.business.clabeLast4}` : 'CLABE pendiente';
  const displayBrandName = profile.business.name.trim() || profile.name;
  const filterCounts: Record<ChargeFilter, number> = {
    all: allCharges.length,
    pending: allCharges.filter((charge) => charge.status === 'pending').length,
    overdue: allCharges.filter((charge) => charge.overdue).length,
    paid: allCharges.filter((charge) => charge.status === 'paid').length,
    cancelled: allCharges.filter((charge) => charge.status === 'cancelled').length,
  };

  function renderNavIcon(id: DashboardScreen) {
    if (id === 'payment') return <BoltIcon />;
    if (id === 'charges') return <ListIcon />;
    if (id === 'clients') return <UsersIcon />;
    return <HomeIcon />;
  }

  return (
    <section className="db-shell" aria-labelledby="db-business-title">
      <h1 className="db-sr-only">Panel de cobros — {displayBrandName}</h1>

      <header className="db-header">
        <div className="db-brand">
          <div className="db-avatar" aria-hidden="true">
            {displayBrandName.charAt(0).toUpperCase()}
          </div>
          <div className="db-brand-copy">
            <div className="db-brand-title-row">
              <strong id="db-business-title" className="db-brand-name">
                {displayBrandName}
              </strong>
              <span className="db-clabe-pill">{clabeLabel}</span>
            </div>
            <p className="db-brand-sub">
              {profile.name} · {profile.profession}
            </p>
          </div>
        </div>

        <div className="db-header-actions">
          <button
            type="button"
            className="db-btn-accent db-desktop-charge-cta"
            onClick={() => setScreen('payment')}
          >
            <PlusIcon />
            <span>Cobrar</span>
          </button>
          <button
            type="button"
            className="db-btn-secondary"
            aria-label="Abrir perfil del negocio"
            aria-current={screen === 'profile' ? 'page' : undefined}
            onClick={() => setScreen('profile')}
          >
            <UserIcon />
            <span>Perfil</span>
          </button>
        </div>
      </header>

      <nav className="db-desktop-nav" aria-label="Secciones principales">
        {NAV_TABS.map((tab) => (
          <button
            key={tab.id}
            type="button"
            className="db-nav-tab"
            aria-current={screen === tab.id ? 'page' : undefined}
            onClick={() => setScreen(tab.id)}
          >
            {renderNavIcon(tab.id)}
            <span>{tab.label}</span>
          </button>
        ))}
      </nav>

      {toast && (
        <div role="status" className="db-toast">
          <CheckIcon />
          <span>{toast}</span>
        </div>
      )}

      <div className="db-body">
        {screen === 'dashboard' && allCharges.length === 0 && (
          <FirstChargeStepper
            profile={profile}
            customers={customers.customers}
            customerPhones={customerPhones}
            clabeLabel={clabeLabel}
            onCreateCharge={handleCreateCharge}
            onRegisterCustomer={() => setScreen('clients')}
            onNotify={notify}
            onCreated={(result) => {
              // Show the new charge as the first row of the payments table.
              setFilter('all');
              setSearch('');
              setHighlightedChargeId(result.charge.chargeId);
              setScreen('charges');
            }}
          />
        )}

        {screen === 'dashboard' && allCharges.length > 0 && (
          <DashboardOverview
            summary={summary}
            recentCharges={allCharges}
            onNavigate={setScreen}
            onOpenChargesWithFilter={(nextFilter) => {
              setFilter(nextFilter);
              setScreen('charges');
            }}
          />
        )}

        {screen === 'payment' && (
          <QuickPaymentsView
            profile={profile}
            customers={customers.customers}
            customerDetails={customerDetails}
            selectedCustomerId={selectedCustomerId}
            clabeLabel={clabeLabel}
            onSelectCustomer={setSelectedCustomerId}
            onCreateCharge={handleCreateCharge}
            onGoToClients={() => setScreen('clients')}
            onGoToCharges={() => {
              setFilter('all');
              setScreen('charges');
            }}
            onNotify={notify}
          />
        )}

        {screen === 'clients' && (
          <CustomersView
            customersData={customers}
            summary={summary}
            customerDetails={customerDetails}
            onCreateCustomer={handleCreateCustomer}
            onUpdateCustomer={handleUpdateCustomer}
            onChargeCustomer={(customerId) => {
              setSelectedCustomerId(customerId);
              setScreen('payment');
            }}
            onViewCustomerCharges={(customerName) => {
              setSearch(customerName);
              setFilter('all');
              setScreen('charges');
            }}
          />
        )}

        {screen === 'charges' && (
          <PaymentsListView
            charges={charges}
            filter={filter}
            filterCounts={filterCounts}
            search={search}
            onFilterChange={setFilter}
            onSearchChange={setSearch}
            onOpenAction={openActionModal}
            onCreatePayment={() => setScreen(allCharges.length === 0 ? 'dashboard' : 'payment')}
            highlightedChargeId={highlightedChargeId}
          />
        )}

        {screen === 'profile' && (
          <ProfileView
            profile={profile}
            clabeLabel={clabeLabel}
            demo={demo}
            onUpdateProfile={
              onUpdateProfile
                ? async (patch) => {
                    await onUpdateProfile(patch);
                    notify('Perfil actualizado correctamente.');
                    reload();
                  }
                : undefined
            }
            onSimulatePayment={() => {
              const chargeId = demo?.simulateProviderPayment(contractorId);
              notify(chargeId ? `Webhook CoDi simulado: ${chargeId} marcado como pagado.` : 'No hay cobros pendientes por simular.');
              reload();
            }}
            onSimulateConflict={() => {
              const target = allCharges.find((charge) => charge.status === 'pending');
              if (!demo || !target) {
                notify('Necesitas al menos un cobro pendiente para simular un conflicto.');
                return;
              }
              demo.simulateConcurrentEdit(target.chargeId, contractorId);
              notify(`Otro dispositivo editó "${target.concept}". Tu próximo cambio recibirá 409 version_conflict.`);
            }}
            onResetDemo={() => {
              demo?.reset(contractorId);
              setFilter('all');
              setSearch('');
              setHighlightedChargeId(null);
              notify('Datos vaciados. En Inicio verás el asistente de primer cobro.');
              reload();
            }}
            onLoadDemo={() => {
              demo?.loadDemoData(contractorId);
              setFilter('all');
              setSearch('');
              notify('Se cargaron 5 cobros de prueba.');
              reload();
            }}
            onSignOut={onSignOut}
          />
        )}
      </div>

      <nav className="db-bottom-nav" aria-label="Navegación móvil">
        {NAV_TABS.map((tab) => (
          <button
            key={tab.id}
            type="button"
            className="db-bottom-btn"
            aria-current={screen === tab.id ? 'page' : undefined}
            onClick={() => setScreen(tab.id)}
          >
            {renderNavIcon(tab.id)}
            <span>{tab.label}</span>
          </button>
        ))}
      </nav>

      <ChargeActionModal
        modal={modal}
        modalErrors={modalErrors}
        busy={busy}
        onClose={() => setModal(null)}
        onSendReminder={handleSendReminder}
        onManualPayment={handleManualPayment}
        onCancelCharge={handleCancelCharge}
        onEditCharge={handleEditCharge}
      />
    </section>
  );
}
