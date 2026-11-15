import type {
  ChargeField,
  ChargesQuery,
  CustomerRowDto,
  CustomerStanding,
  DashboardChargeDto,
  DashboardErrorCode,
  DashboardProfileDto,
  DashboardSummaryDto,
  PaymentMethod,
  UpdateDashboardChargeRequest,
  VendorClientDto,
} from '@platform/contracts';
import {
  DashboardApiError,
  type CreatedChargeResult,
  type CustomersDirectoryGateway,
  type DashboardGateway,
  type NewChargeInput,
  type PaymentsGateway,
} from '../application/dashboard-gateway';
import {
  AMOUNT_MAX_CENTS,
  AMOUNT_MIN_CENTS,
  CONCEPT_MAX_LENGTH,
  DASHBOARD_TIMEZONE,
  MANUAL_PAYMENT_METHODS,
  MAX_TAGS,
  NOTE_MAX_LENGTH,
  TAG_MAX_LENGTH,
  normalizeNewCustomerInput,
  todayInMexico,
  validateNewCustomer,
  type CustomerBusinessDetails,
  type CustomerEntityType,
  type NewCustomerInput,
} from '../domain/dashboard';

/*
 * Persistent local implementation (localStorage + in-memory fallback) of the
 * dashboard, customer directory (individual & company/merchant details), and
 * payments contract while backend endpoints beyond GET /profile are built.
 */

export const MOCK_CONTRACTOR_ID = 'demo';

const STORAGE_PREFIX = 'contractors-platform.workspace.';

const EDITABLE_FIELDS: Record<DashboardChargeDto['status'], ChargeField[]> = {
  pending: ['concept', 'dueDate', 'amountCents', 'note', 'tags'],
  paid: ['note', 'tags'],
  cancelled: ['note', 'tags'],
};
const ALL_METHODS: PaymentMethod[] = ['codi', 'card', 'cash', 'transfer', 'other'];
const CARD_FEE = { feeRate: 0.036, fixedFeeCents: 300, vatRate: 0.16 };
const FREE_PLAN_CUSTOMER_LIMIT = 5;
const MAX_TOP_DEBTORS = 5;
const DAY_MS = 86_400_000;

const MESSAGES = {
  chargeNotFound: 'No se encontró el cobro.',
  customerNotFound: 'No se encontró el cliente.',
  invalidCharge: 'Revisa los datos del cobro.',
  invalidCustomer: 'Revisa los datos del cliente.',
  versionConflict: 'El cobro cambió mientras lo editabas. Revisa la versión actual.',
  fieldNotEditable: 'Este dato ya no se puede modificar.',
  chargeNotPayable: 'Este cobro ya está pagado o cancelado.',
  chargeNotCancellable: 'Solo puedes cancelar cobros pendientes.',
} as const;

interface StoredCustomer {
  readonly customerId: string;
  readonly entityType?: CustomerEntityType;
  readonly name: string;
  readonly phone: string;
  readonly email: string;
  readonly business?: CustomerBusinessDetails;
}

type StoredCharge = Omit<DashboardChargeDto, 'overdue' | 'editableFields' | 'customerName'>;

interface WorkspaceState {
  customers: StoredCustomer[];
  charges: StoredCharge[];
  sequence: number;
  customerSequence: number;
}

function addDays(date: string, days: number): string {
  return new Date(Date.parse(`${date}T00:00:00Z`) + days * DAY_MS).toISOString().slice(0, 10);
}
const instant = (date: string, time = '17:00:00') => `${date}T${time}-06:00`;
const daysBetween = (from: string, to: string) => Math.round((Date.parse(to) - Date.parse(from)) / DAY_MS);

/** A new business starts with no customers or charges (first-charge stepper). */
function emptyState(): WorkspaceState {
  return { customers: [], charges: [], sequence: 0, customerSequence: 0 };
}

/** Five sample charges, one per state, loaded on demand from the profile. */
function seed(today: string): WorkspaceState {
  const customers: StoredCustomer[] = [
    {
      customerId: 'cus_norte',
      entityType: 'company',
      name: 'Sofía Ramírez',
      phone: '5587654321',
      email: 'pagos@norteindustrial.mx',
      business: {
        legalName: 'Comercializadora del Norte S.A. de C.V.',
        tradeName: 'Norte Industrial',
        rfc: 'CNO180412AB1',
        activity: 'Comercio y abarrotes',
        taxRegime: '601 - General de Ley Personas Morales',
        fiscalAddress: 'Av. Reforma 450, Col. Juárez, CDMX',
      },
    },
    {
      customerId: 'cus_mateo',
      entityType: 'individual',
      name: 'Mateo Hernández',
      phone: '5541238910',
      email: 'mateo@ejemplo.mx',
    },
    {
      customerId: 'cus_roble',
      entityType: 'company',
      name: 'Diego Castellanos',
      phone: '5533445566',
      email: 'facturacion@elroble.mx',
      business: {
        legalName: 'Ferretería y Materiales El Roble',
        tradeName: 'El Roble',
        rfc: 'CADG8905121H2',
        activity: 'Construcción y remodelación',
        taxRegime: '626 - Régimen Simplificado de Confianza (RESICO)',
        fiscalAddress: 'Calz. Tlalpan 1120, Benito Juárez, CDMX',
      },
    },
    {
      customerId: 'cus_lucia',
      entityType: 'individual',
      name: 'Lucía Mendoza',
      phone: '5599887766',
      email: 'lucia@ejemplo.mx',
    },
    {
      customerId: 'cus_andres',
      entityType: 'individual',
      name: 'Andrés Villalobos',
      phone: '5522334455',
      email: 'andres@ejemplo.mx',
    },
  ];
  const base = (id: string, customerId: string, concept: string, amountCents: number, createdDaysAgo: number, dueInDays: number): StoredCharge => ({
    chargeId: id,
    version: 1,
    customerId,
    concept,
    amountCents,
    dueDate: addDays(today, dueInDays),
    status: 'pending',
    createdAt: instant(addDays(today, -createdDaysAgo), '09:00:00'),
    updatedAt: instant(addDays(today, -createdDaysAgo), '09:00:00'),
    paidAt: null,
    paymentMethod: null,
    cancelledAt: null,
    reminderCount: 0,
    note: null,
    tags: [],
  });
  const charges: StoredCharge[] = [
    { ...base('ch_001', 'cus_mateo', 'Mensualidad', 85_000, 2, 6), tags: ['mensualidad'] },
    { ...base('ch_002', 'cus_norte', 'Suministro mensual de refacciones', 185_000, 20, -9), reminderCount: 2, tags: ['empresa', 'refacciones'] },
    {
      ...base('ch_003', 'cus_roble', 'Mantenimiento eléctrico sucursal', 90_000, 8, -1),
      status: 'paid',
      paidAt: instant(addDays(today, -3), '12:30:00'),
      paymentMethod: 'codi',
      version: 2,
      updatedAt: instant(addDays(today, -3), '12:30:00'),
    },
    {
      ...base('ch_004', 'cus_lucia', 'Uniforme e inscripción', 120_000, 10, -4),
      status: 'paid',
      paidAt: instant(addDays(today, -5), '18:10:00'),
      paymentMethod: 'cash',
      reminderCount: 1,
      note: 'Pagó en efectivo al terminar la clase.',
      version: 2,
      updatedAt: instant(addDays(today, -5), '18:10:00'),
    },
    {
      ...base('ch_005', 'cus_andres', 'Clínica sabatina', 35_000, 12, -2),
      status: 'cancelled',
      cancelledAt: instant(addDays(today, -6), '10:00:00'),
      note: 'Se suspendió la clínica por lluvia.',
      version: 2,
      updatedAt: instant(addDays(today, -6), '10:00:00'),
    },
  ];
  return { customers, charges, sequence: charges.length, customerSequence: customers.length };
}

const MOCK_PROFILE: DashboardProfileDto = {
  contractorId: MOCK_CONTRACTOR_ID,
  name: 'Carlos Morales',
  profession: 'Profesor de básquetbol',
  business: {
    name: 'Academia Hoops',
    activity: 'Deportes y entrenamiento',
    rfc: 'MOCC880101AB1',
    taxRegime: 'RESICO',
    taxpayerType: 'individual',
    clabeLast4: '7771',
  },
  firstCustomer: { name: 'Mateo Hernández', phone: '5541238910', email: 'mateo@ejemplo.mx' },
  onboarding: { status: 'complete', pendingFields: [] },
};

function fail(status: number, message: string, code?: DashboardErrorCode, fields?: Record<string, string>): never {
  throw new DashboardApiError(message, status, code, fields);
}

const normalize = (value: string) => value.normalize('NFD').replace(/\p{Diacritic}/gu, '').toLowerCase();

function toVendorClient(contractorId: string, customer: StoredCustomer): VendorClientDto {
  const now = new Date().toISOString();
  return {
    clientId: customer.customerId,
    vendorId: contractorId,
    entityType: customer.entityType ?? 'individual',
    name: customer.name,
    phone: customer.phone,
    email: customer.email,
    ...(customer.business?.legalName ? { legalName: customer.business.legalName } : {}),
    ...(customer.business?.tradeName ? { tradeName: customer.business.tradeName } : {}),
    ...(customer.business?.rfc ? { rfc: customer.business.rfc } : {}),
    ...(customer.business?.activity ? { activity: customer.business.activity } : {}),
    ...(customer.business?.taxRegime ? { taxRegime: customer.business.taxRegime } : {}),
    ...(customer.business?.fiscalAddress ? { fiscalAddress: customer.business.fiscalAddress } : {}),
    createdAt: now,
    updatedAt: now,
  };
}

function customerDisplayName(customer: StoredCustomer): string {
  const entityType = customer.entityType ?? 'individual';
  if (entityType === 'company' && customer.business?.legalName) {
    return customer.business.tradeName || customer.business.legalName;
  }
  return customer.name;
}

function loadStoredState(contractorId: string, fallbackToday: string): WorkspaceState {
  if (typeof window === 'undefined') return seed(fallbackToday);
  try {
    const raw = window.localStorage.getItem(`${STORAGE_PREFIX}${contractorId}`);
    if (!raw) return emptyState();
    const parsed = JSON.parse(raw) as Partial<WorkspaceState>;
    if (!Array.isArray(parsed.customers) || !Array.isArray(parsed.charges)) {
      return emptyState();
    }
    return {
      customers: parsed.customers,
      charges: parsed.charges,
      sequence: typeof parsed.sequence === 'number' ? parsed.sequence : parsed.charges.length,
      customerSequence: typeof parsed.customerSequence === 'number' ? parsed.customerSequence : parsed.customers.length,
    };
  } catch {
    return emptyState();
  }
}

function persistState(contractorId: string, state: WorkspaceState) {
  if (typeof window === 'undefined') return;
  try {
    window.localStorage.setItem(`${STORAGE_PREFIX}${contractorId}`, JSON.stringify(state));
  } catch {
    /* Ignore storage quota errors in local prototype mode. */
  }
}

export interface MockDashboard extends DashboardGateway, PaymentsGateway, CustomersDirectoryGateway {
  /** Simulates the pey webhook paying the oldest pending charge with CoDi. */
  simulateProviderPayment(contractorId?: string): string | null;
  /** Simulates another device editing a charge, so the next write gets 409. */
  simulateConcurrentEdit(chargeId: string, contractorId?: string): void;
  /** Empties the business: no customers or charges. */
  reset(contractorId?: string): void;
  /** Replaces the business data with the five sample charges. */
  loadDemoData(contractorId?: string): void;
}

export function createMockDashboard(): MockDashboard {
  const states = new Map<string, WorkspaceState>();
  const today = () => todayInMexico();
  const now = () => new Date().toISOString();

  function getState(contractorId: string): WorkspaceState {
    const existing = states.get(contractorId);
    if (existing) return existing;
    const loaded = loadStoredState(contractorId, today());
    states.set(contractorId, loaded);
    return loaded;
  }

  function saveState(contractorId: string, next: WorkspaceState) {
    states.set(contractorId, next);
    persistState(contractorId, next);
  }

  function customerName(state: WorkspaceState, id: string): string {
    const customer = state.customers.find((c) => c.customerId === id);
    return customer ? customerDisplayName(customer) : 'Cliente';
  }

  const isOverdue = (charge: StoredCharge) => charge.status === 'pending' && charge.dueDate < today();

  function view(state: WorkspaceState, charge: StoredCharge): DashboardChargeDto {
    return {
      ...charge,
      tags: [...charge.tags],
      customerName: customerName(state, charge.customerId),
      overdue: isOverdue(charge),
      editableFields: [...EDITABLE_FIELDS[charge.status]],
    };
  }

  function findCharge(state: WorkspaceState, chargeId: string): StoredCharge {
    return state.charges.find((charge) => charge.chargeId === chargeId) ?? fail(404, MESSAGES.chargeNotFound);
  }

  function checkVersion(charge: StoredCharge, version: number) {
    if (charge.version !== version) fail(409, MESSAGES.versionConflict, 'version_conflict');
  }

  function saveCharge(contractorId: string, next: StoredCharge): DashboardChargeDto {
    const state = getState(contractorId);
    const stored = { ...next, version: next.version + 1, updatedAt: now() };
    const nextState: WorkspaceState = {
      ...state,
      charges: state.charges.map((charge) => (charge.chargeId === stored.chargeId ? stored : charge)),
    };
    saveState(contractorId, nextState);
    return view(nextState, stored);
  }

  function validate(body: UpdateDashboardChargeRequest) {
    const errors: Record<string, string> = {};
    if (body.concept !== undefined && (!body.concept.trim() || body.concept.trim().length > CONCEPT_MAX_LENGTH)) {
      errors.concept = `Usa entre 1 y ${CONCEPT_MAX_LENGTH} caracteres.`;
    }
    if (body.amountCents !== undefined && (!Number.isInteger(body.amountCents) || body.amountCents < AMOUNT_MIN_CENTS || body.amountCents > AMOUNT_MAX_CENTS)) {
      errors.amountCents = 'Ingresa un monto válido.';
    }
    if (body.dueDate !== undefined && !/^\d{4}-\d{2}-\d{2}$/.test(body.dueDate)) errors.dueDate = 'Usa el formato AAAA-MM-DD.';
    if (body.note && body.note.length > NOTE_MAX_LENGTH) errors.note = `Usa como máximo ${NOTE_MAX_LENGTH} caracteres.`;
    if (body.tags && (body.tags.length > MAX_TAGS || body.tags.some((tag) => tag.length > TAG_MAX_LENGTH))) {
      errors.tags = `Hasta ${MAX_TAGS} etiquetas de ${TAG_MAX_LENGTH} caracteres.`;
    }
    if (Object.keys(errors).length) fail(422, MESSAGES.invalidCharge, undefined, errors);
  }

  function buildCustomerRow(state: WorkspaceState, customer: StoredCustomer): CustomerRowDto {
    const own = state.charges.filter((charge) => charge.customerId === customer.customerId);
    const pending = own.filter((charge) => charge.status === 'pending');
    const overdue = pending.filter(isOverdue);
    const upcoming = pending.filter((charge) => !isOverdue(charge)).map((charge) => charge.dueDate).sort();
    const lastPaid = own
      .filter((charge) => charge.status === 'paid')
      .sort((a, b) => (b.paidAt ?? '').localeCompare(a.paidAt ?? ''))[0];
    const standing: CustomerStanding = overdue.length ? 'overdue' : pending.length ? 'pending' : 'up_to_date';
    return {
      customerId: customer.customerId,
      name: customerDisplayName(customer),
      standing,
      balance: {
        outstandingCents: pending.reduce((sum, charge) => sum + charge.amountCents, 0),
        overdueCents: overdue.reduce((sum, charge) => sum + charge.amountCents, 0),
      },
      lastPayment: lastPaid ? { amountCents: lastPaid.amountCents, paidAt: lastPaid.paidAt!, method: lastPaid.paymentMethod! } : null,
      nextDueDate: upcoming[0] ?? null,
      oldestDueDate: overdue.map((charge) => charge.dueDate).sort()[0] ?? null,
    };
  }

  const gateway: MockDashboard = {
    async profile(contractorId) {
      return { ...MOCK_PROFILE, contractorId };
    },

    async list(contractorId) {
      return getState(contractorId).customers.map((customer) => toVendorClient(contractorId, customer));
    },

    async createCustomer(contractorId, input: NewCustomerInput) {
      const normalized = normalizeNewCustomerInput(input);
      const errors = validateNewCustomer(normalized);
      if (Object.keys(errors).length) {
        fail(422, MESSAGES.invalidCustomer, undefined, errors as Record<string, string>);
      }
      const state = getState(contractorId);
      const nextSequence = state.customerSequence + 1;
      const created: StoredCustomer = {
        customerId: `cus_${String(nextSequence).padStart(3, '0')}`,
        entityType: normalized.entityType,
        name: normalized.name,
        phone: normalized.phone,
        email: normalized.email,
        ...(normalized.business ? { business: normalized.business } : {}),
      };
      const nextState: WorkspaceState = {
        ...state,
        customerSequence: nextSequence,
        customers: [created, ...state.customers],
      };
      saveState(contractorId, nextState);
      return toVendorClient(contractorId, created);
    },

    async updateCustomer(contractorId, customerId, input: NewCustomerInput) {
      const normalized = normalizeNewCustomerInput(input);
      const errors = validateNewCustomer(normalized);
      if (Object.keys(errors).length) {
        fail(422, MESSAGES.invalidCustomer, undefined, errors as Record<string, string>);
      }
      const state = getState(contractorId);
      const target = state.customers.find((customer) => customer.customerId === customerId);
      if (!target) fail(404, MESSAGES.customerNotFound);

      const updatedCustomer: StoredCustomer = {
        customerId,
        entityType: normalized.entityType,
        name: normalized.name,
        phone: normalized.phone,
        email: normalized.email,
        ...(normalized.business ? { business: normalized.business } : {}),
      };
      const nextState: WorkspaceState = {
        ...state,
        customers: state.customers.map((customer) => (customer.customerId === customerId ? updatedCustomer : customer)),
      };
      saveState(contractorId, nextState);
      return toVendorClient(contractorId, updatedCustomer);
    },

    async summary(contractorId, query = {}) {
      const state = getState(contractorId);
      const to = query.to ?? today();
      const from = query.from ?? `${to.slice(0, 8)}01`;
      const inPeriod = (date: string | null) => !!date && date.slice(0, 10) >= from && date.slice(0, 10) <= to;
      const paid = state.charges.filter((charge) => charge.status === 'paid' && inPeriod(charge.paidAt));
      const issued = state.charges.filter((charge) => charge.status !== 'cancelled' && inPeriod(charge.createdAt));
      const pending = state.charges.filter((charge) => charge.status === 'pending');
      const overdue = pending.filter(isOverdue);
      const sum = (list: StoredCharge[]) => list.reduce((total, charge) => total + charge.amountCents, 0);
      const customersOf = (list: StoredCharge[]) => new Set(list.map((charge) => charge.customerId)).size;
      const codi = paid.filter((charge) => charge.paymentMethod === 'codi');
      const allRows = state.customers.map((customer) => buildCustomerRow(state, customer));
      const debtors = allRows
        .filter((row) => row.balance.overdueCents > 0)
        .sort((a, b) => b.balance.overdueCents - a.balance.overdueCents)
        .slice(0, MAX_TOP_DEBTORS);

      return {
        contractorId,
        period: { from, to, timezone: DASHBOARD_TIMEZONE },
        currency: 'MXN',
        generatedAt: now(),
        collected: { amountCents: sum(paid), count: paid.length, changePercent: null },
        outstanding: { amountCents: sum(pending), count: pending.length, customerCount: customersOf(pending) },
        overdue: {
          amountCents: sum(overdue),
          count: overdue.length,
          customerCount: customersOf(overdue),
          oldestDaysOverdue: overdue.length ? Math.max(...overdue.map((charge) => daysBetween(charge.dueDate, today()))) : null,
        },
        collectionRate: {
          ratio: issued.length ? issued.filter((charge) => charge.status === 'paid').length / issued.length : null,
          paidCount: issued.filter((charge) => charge.status === 'paid').length,
          issuedCount: issued.length,
        },
        averageDaysToPay: paid.length
          ? paid.reduce((total, charge) => total + daysBetween(charge.createdAt.slice(0, 10), charge.paidAt!.slice(0, 10)), 0) / paid.length
          : null,
        reminders: {
          sent: state.charges.reduce((total, charge) => total + charge.reminderCount, 0),
          paidWithoutReminderRatio: paid.length ? paid.filter((charge) => charge.reminderCount === 0).length / paid.length : null,
        },
        commissionSavings: {
          amountCents: Math.round(codi.reduce((total, charge) => total + (charge.amountCents * CARD_FEE.feeRate + CARD_FEE.fixedFeeCents) * (1 + CARD_FEE.vatRate), 0)),
          assumption: CARD_FEE,
        },
        paymentMethods: ALL_METHODS.map((method) => {
          const list = paid.filter((charge) => charge.paymentMethod === method);
          return { method, amountCents: sum(list), count: list.length };
        }),
        customers: { active: state.customers.length, plan: 'free', limit: FREE_PLAN_CUSTOMER_LIMIT },
        topDebtors: debtors.map((row) => ({
          customerId: row.customerId,
          name: row.name,
          amountCents: row.balance.overdueCents,
          overdueCount: state.charges.filter((charge) => charge.customerId === row.customerId && isOverdue(charge)).length,
          oldestDueDate: row.oldestDueDate!,
        })),
      } satisfies DashboardSummaryDto;
    },

    async charges(contractorId, query: ChargesQuery = {}) {
      const state = getState(contractorId);
      const term = query.search ? normalize(query.search) : '';
      const list = state.charges
        .filter((charge) => !query.status || charge.status === query.status)
        .filter((charge) => query.overdue === undefined || isOverdue(charge) === query.overdue)
        .filter((charge) => !query.customerId || charge.customerId === query.customerId)
        .filter((charge) => !query.tag || charge.tags.includes(query.tag))
        .filter((charge) => !term || normalize(charge.concept).includes(term) || normalize(customerName(state, charge.customerId)).includes(term))
        .sort((a, b) => {
          if (query.sort === 'amount') return b.amountCents - a.amountCents;
          if (query.sort === 'created') return b.createdAt.localeCompare(a.createdAt);
          return a.dueDate.localeCompare(b.dueDate);
        })
        .map((charge) => view(state, charge));
      return {
        charges: list,
        totals: { count: list.length, amountCents: list.reduce((total, charge) => total + charge.amountCents, 0) },
        nextCursor: null,
      };
    },

    async customers(contractorId, query = {}) {
      const state = getState(contractorId);
      const term = query.search ? normalize(query.search) : '';
      const matchesTerm = (customer: StoredCustomer) => {
        if (!term) return true;
        const fields = [
          customer.name,
          customer.email,
          customer.phone,
          customer.business?.legalName,
          customer.business?.tradeName,
          customer.business?.rfc,
          customer.business?.activity,
        ];
        return fields.some((value) => value && normalize(value).includes(term));
      };
      const all = state.customers
        .filter(matchesTerm)
        .map((customer) => buildCustomerRow(state, customer));
      return {
        customers: all.filter((row) => !query.standing || row.standing === query.standing),
        counts: {
          all: all.length,
          overdue: all.filter((row) => row.standing === 'overdue').length,
          pending: all.filter((row) => row.standing === 'pending').length,
          upToDate: all.filter((row) => row.standing === 'up_to_date').length,
        },
        nextCursor: null,
      };
    },

    async updateCharge(contractorId, chargeId, body) {
      const state = getState(contractorId);
      const charge = findCharge(state, chargeId);
      checkVersion(charge, body.version);
      const { version: _version, ...changes } = body;
      for (const field of Object.keys(changes) as ChargeField[]) {
        if (changes[field] !== undefined && !EDITABLE_FIELDS[charge.status].includes(field)) {
          fail(422, MESSAGES.fieldNotEditable, 'field_not_editable', { [field]: MESSAGES.fieldNotEditable });
        }
      }
      validate(body);
      return saveCharge(contractorId, {
        ...charge,
        concept: changes.concept?.trim() ?? charge.concept,
        dueDate: changes.dueDate ?? charge.dueDate,
        amountCents: changes.amountCents ?? charge.amountCents,
        note: changes.note === undefined ? charge.note : changes.note?.trim() || null,
        tags: changes.tags ?? charge.tags,
      });
    },

    async recordManualPayment(contractorId, chargeId, body) {
      const state = getState(contractorId);
      const charge = findCharge(state, chargeId);
      checkVersion(charge, body.version);
      if (charge.status !== 'pending') fail(409, MESSAGES.chargeNotPayable, 'charge_not_payable');
      if (!MANUAL_PAYMENT_METHODS.includes(body.method)) fail(422, MESSAGES.invalidCharge, undefined, { method: 'Elige un método válido.' });
      return saveCharge(contractorId, {
        ...charge,
        status: 'paid',
        paymentMethod: body.method,
        paidAt: body.paidAt ?? now(),
        note: body.note?.trim() || charge.note,
      });
    },

    async cancelCharge(contractorId, chargeId, body) {
      const state = getState(contractorId);
      const charge = findCharge(state, chargeId);
      checkVersion(charge, body.version);
      if (charge.status !== 'pending') fail(409, MESSAGES.chargeNotCancellable, 'charge_not_cancellable');
      return saveCharge(contractorId, { ...charge, status: 'cancelled', cancelledAt: now(), note: body.reason?.trim() || charge.note });
    },

    async createCharge(contractorId, input: NewChargeInput): Promise<CreatedChargeResult> {
      const state = getState(contractorId);
      if (!state.customers.some((customer) => customer.customerId === input.customerId)) {
        fail(404, MESSAGES.customerNotFound);
      }
      const nextSequence = state.sequence + 1;
      const chargeId = `ch_${String(nextSequence).padStart(3, '0')}`;
      const created: StoredCharge = {
        chargeId,
        version: 1,
        customerId: input.customerId,
        concept: input.concept.trim(),
        amountCents: input.amountCents,
        dueDate: input.dueDate,
        status: 'pending',
        createdAt: now(),
        updatedAt: now(),
        paidAt: null,
        paymentMethod: null,
        cancelledAt: null,
        reminderCount: 0,
        note: null,
        tags: input.method === 'PAYMENT_LINK' ? ['link'] : ['codi'],
      };
      const nextState: WorkspaceState = {
        ...state,
        sequence: nextSequence,
        charges: [created, ...state.charges],
      };
      saveState(contractorId, nextState);
      return {
        charge: view(nextState, created),
        method: input.method,
        ...(input.method === 'PAYMENT_LINK'
          ? { paymentLinkUrl: `https://pay.praxis-demo.mx/c/${chargeId}` }
          : input.method === 'CODI_QR'
            ? { codiQrString: `CODI://pay/${contractorId}/${chargeId}/${input.amountCents}` }
            : {}),
      };
    },

    async sendReminder(contractorId, chargeId) {
      const state = getState(contractorId);
      const charge = findCharge(state, chargeId);
      if (charge.status !== 'pending') fail(409, MESSAGES.chargeNotPayable, 'charge_not_payable');
      return saveCharge(contractorId, { ...charge, reminderCount: charge.reminderCount + 1 });
    },

    simulateProviderPayment(contractorId = MOCK_CONTRACTOR_ID) {
      const state = getState(contractorId);
      const next = state.charges
        .filter((charge) => charge.status === 'pending')
        .sort((a, b) => a.dueDate.localeCompare(b.dueDate))[0];
      if (!next) return null;
      saveCharge(contractorId, { ...next, status: 'paid', paymentMethod: 'codi', paidAt: now() });
      return next.chargeId;
    },

    simulateConcurrentEdit(chargeId, contractorId = MOCK_CONTRACTOR_ID) {
      const state = getState(contractorId);
      const charge = findCharge(state, chargeId);
      saveState(contractorId, {
        ...state,
        charges: state.charges.map((item) =>
          item.chargeId === chargeId ? { ...item, version: item.version + 1, updatedAt: now() } : item,
        ),
      });
    },

    reset(contractorId = MOCK_CONTRACTOR_ID) {
      saveState(contractorId, emptyState());
    },

    loadDemoData(contractorId = MOCK_CONTRACTOR_ID) {
      saveState(contractorId, seed(today()));
    },
  };
  return gateway;
}
