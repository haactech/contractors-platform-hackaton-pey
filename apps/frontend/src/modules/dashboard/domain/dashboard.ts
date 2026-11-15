import type {
  ChargeField,
  ChargesQuery,
  ChargeStatus,
  CustomerStanding,
  DashboardChargeDto,
  ManualPaymentMethod,
  PaymentMethod,
  PaymentMethodDto,
} from '@platform/contracts';

/* Values fixed by the dashboard contract (docs/dashboard-api.md). */
export const CONCEPT_MAX_LENGTH = 40;
export const NOTE_MAX_LENGTH = 500;
export const MAX_TAGS = 10;
export const TAG_MAX_LENGTH = 30;
export const AMOUNT_MIN_CENTS = 1;
export const AMOUNT_MAX_CENTS = 100_000_000;
export const SEARCH_MIN_LENGTH = 2;
export const DASHBOARD_TIMEZONE = 'America/Mexico_City';
export const MX_PHONE_DIGITS = 10;

export const MANUAL_PAYMENT_METHODS: readonly ManualPaymentMethod[] = ['cash', 'transfer', 'other'];

export const QUICK_AMOUNT_PESOS = [250, 500, 1000, 1500] as const;
export const QUICK_CONCEPTS = ['Anticipo', 'Servicio', 'Mensualidad', 'Liquidación'] as const;

export const CUSTOMER_ACTIVITIES = [
  'Comercio y abarrotes',
  'Construcción y remodelación',
  'Alimentos y bebidas',
  'Servicios profesionales y consultoría',
  'Salud y bienestar',
  'Educación y capacitación',
  'Tecnología y soporte',
  'Transporte y logística',
  'Manufactura y talleres',
  'Otro',
] as const;

export const CUSTOMER_TAX_REGIMES = [
  '601 - General de Ley Personas Morales',
  '603 - Personas Morales con Fines no Lucrativos',
  '612 - Personas Físicas con Actividades Empresariales',
  '626 - Régimen Simplificado de Confianza (RESICO)',
  '621 - Incorporación Fiscal',
] as const;

export const QUICK_PAYMENT_METHODS: readonly {
  readonly id: PaymentMethodDto;
  readonly title: string;
  readonly description: string;
}[] = [
  {
    id: 'PAYMENT_LINK',
    title: 'Enlace de pago',
    description: 'Envía por WhatsApp, correo o copia la liga directa.',
  },
  {
    id: 'CODI_QR',
    title: 'QR CoDi',
    description: 'Cobra al instante en persona sin comisiones.',
  },
  {
    id: 'CODI_PUSH',
    title: 'CoDi al teléfono',
    description: 'Envía el cobro directo al teléfono del cliente.',
  },
];

export type DashboardScreen = 'dashboard' | 'payment' | 'charges' | 'clients' | 'profile';

/** Table tabs. Each maps to a ChargesQuery of the contract. */
export type ChargeFilter = 'all' | 'pending' | 'overdue' | 'paid' | 'cancelled';

export const CHARGE_FILTERS: readonly { readonly id: ChargeFilter; readonly label: string }[] = [
  { id: 'all', label: 'Todos' },
  { id: 'pending', label: 'Pendientes' },
  { id: 'overdue', label: 'Vencidos' },
  { id: 'paid', label: 'Pagados' },
  { id: 'cancelled', label: 'Cancelados' },
];

export function chargesQueryFor(filter: ChargeFilter, search: string): ChargesQuery {
  const query: ChargesQuery = {};
  if (filter === 'overdue') query.overdue = true;
  else if (filter !== 'all') query.status = filter;
  const term = search.trim();
  if (term.length >= SEARCH_MIN_LENGTH) query.search = term;
  return query;
}

/** Visual state of a charge: `overdue` is a flag on pending charges, not a status. */
export type ChargeBadge = 'pending' | 'overdue' | 'paid' | 'cancelled';

export function chargeBadge(charge: Pick<DashboardChargeDto, 'status' | 'overdue'>): ChargeBadge {
  if (charge.status === 'pending' && charge.overdue) return 'overdue';
  return charge.status;
}

export const CHARGE_BADGE_LABELS: Record<ChargeBadge, string> = {
  pending: 'Pendiente',
  overdue: 'Vencido',
  paid: 'Pagado',
  cancelled: 'Cancelado',
};

export const CHARGE_STATUS_LABELS: Record<ChargeStatus, string> = {
  pending: 'Pendiente',
  paid: 'Pagado',
  cancelled: 'Cancelado',
};

export const PAYMENT_METHOD_LABELS: Record<PaymentMethod, string> = {
  codi: 'CoDi',
  card: 'Tarjeta',
  cash: 'Efectivo',
  transfer: 'Transferencia',
  other: 'Otro',
};

export const STANDING_LABELS: Record<CustomerStanding, string> = {
  overdue: 'Vencido',
  pending: 'Pendiente',
  up_to_date: 'Al día',
};

export const CHARGE_FIELD_LABELS: Record<ChargeField, string> = {
  concept: 'Concepto',
  dueDate: 'Fecha límite',
  amountCents: 'Monto',
  note: 'Nota privada',
  tags: 'Etiquetas',
};

export type CustomerEntityType = 'individual' | 'company';

export const CUSTOMER_ENTITY_LABELS: Record<CustomerEntityType, string> = {
  individual: 'Persona física',
  company: 'Persona moral / Comercio',
};

export interface CustomerBusinessDetails {
  /** Legal corporate name or merchant name (required when entityType === 'company'). */
  legalName: string;
  /** Commercial brand name when it differs from legalName. */
  tradeName?: string;
  /** Mexican RFC: 12 chars for companies, 13 for individuals with business activity. */
  rfc?: string;
  /** Line of business or activity. */
  activity?: string;
  /** SAT tax regime label. */
  taxRegime?: string;
  /** Fiscal address, branch location, or billing note. */
  fiscalAddress?: string;
}

export interface CustomerProfileRecord {
  customerId: string;
  entityType: CustomerEntityType;
  /** Person name (for individuals) or main payment contact (for companies/merchants). */
  name: string;
  phone: string;
  email: string;
  business?: CustomerBusinessDetails;
}

export interface NewCustomerInput {
  entityType: CustomerEntityType;
  name: string;
  phone: string;
  email: string;
  business?: CustomerBusinessDetails;
}

export type CustomerFieldName =
  | 'name'
  | 'phone'
  | 'email'
  | 'legalName'
  | 'tradeName'
  | 'rfc'
  | 'activity'
  | 'taxRegime'
  | 'fiscalAddress';

export type CustomerFieldErrors = Partial<Record<CustomerFieldName, string>>;

const RFC_PATTERN = /^[A-ZÑ&]{3,4}\d{6}[A-Z0-9]{3}$/;

/** Keeps up to 10 Mexican mobile digits; drops a pasted +52 / 52 prefix. */
export function mxPhoneDigits(value: string): string {
  let digits = value.replace(/\D/g, '');
  if (digits.length > MX_PHONE_DIGITS && digits.startsWith('52')) digits = digits.slice(2);
  return digits.slice(0, MX_PHONE_DIGITS);
}

/** Formats 10 national digits into "55 4123 8910". */
export function formatMxPhone(digits: string): string {
  return [digits.slice(0, 2), digits.slice(2, 6), digits.slice(6, 10)].filter(Boolean).join(' ');
}

/** Returns SAT classification label based on RFC length when present. */
export function rfcKindLabel(rfc?: string): string | null {
  if (!rfc) return null;
  if (rfc.length === 12) return 'Persona moral (12)';
  if (rfc.length === 13) return 'Persona física (13)';
  return null;
}

/** Normalizes optional business fields and trims whitespace before validation/storage. */
export function normalizeNewCustomerInput(input: NewCustomerInput): NewCustomerInput {
  const rawBiz = input.business;
  const legalName = rawBiz?.legalName.trim() ?? '';
  const tradeName = rawBiz?.tradeName?.trim() || undefined;
  const rfc = rawBiz?.rfc?.trim().toUpperCase() || undefined;
  const activity = rawBiz?.activity?.trim() || undefined;
  const taxRegime = rawBiz?.taxRegime?.trim() || undefined;
  const fiscalAddress = rawBiz?.fiscalAddress?.trim() || undefined;

  const hasAnyBusiness = Boolean(legalName || tradeName || rfc || activity || taxRegime || fiscalAddress);
  const business: CustomerBusinessDetails | undefined =
    input.entityType === 'company' || hasAnyBusiness
      ? {
          legalName,
          ...(tradeName ? { tradeName } : {}),
          ...(rfc ? { rfc } : {}),
          ...(activity ? { activity } : {}),
          ...(taxRegime ? { taxRegime } : {}),
          ...(fiscalAddress ? { fiscalAddress } : {}),
        }
      : undefined;

  return {
    entityType: input.entityType,
    name: input.name.trim(),
    phone: mxPhoneDigits(input.phone),
    email: input.email.trim(),
    ...(business ? { business } : {}),
  };
}

export function validateNewCustomer(input: NewCustomerInput): CustomerFieldErrors {
  const normalized = normalizeNewCustomerInput(input);
  const errors: CustomerFieldErrors = {};

  if (!normalized.name) {
    errors.name =
      normalized.entityType === 'company'
        ? 'Ingresa el nombre del contacto o responsable de pagos.'
        : 'Ingresa el nombre completo del cliente.';
  } else if (normalized.name.length > 255) {
    errors.name = 'Usa un máximo de 255 caracteres.';
  }

  if (!normalized.phone) {
    errors.phone = 'Ingresa el teléfono del cliente.';
  } else if (!/^\d{10}$/.test(normalized.phone)) {
    errors.phone = `Escribe un teléfono de ${MX_PHONE_DIGITS} dígitos.`;
  }

  if (!normalized.email) {
    errors.email = 'Ingresa el correo del cliente.';
  } else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(normalized.email)) {
    errors.email = 'Escribe un correo válido.';
  }

  const biz = normalized.business;
  if (normalized.entityType === 'company') {
    if (!biz?.legalName) {
      errors.legalName = 'Ingresa la razón social o nombre del negocio.';
    } else if (biz.legalName.length > 255) {
      errors.legalName = 'Usa un máximo de 255 caracteres.';
    }
  } else if (biz?.legalName && biz.legalName.length > 255) {
    errors.legalName = 'Usa un máximo de 255 caracteres.';
  }

  if (biz?.tradeName && biz.tradeName.length > 255) errors.tradeName = 'Usa un máximo de 255 caracteres.';
  if (biz?.rfc && !RFC_PATTERN.test(biz.rfc)) {
    errors.rfc = 'Escribe un RFC válido de 12 (empresa) o 13 caracteres (persona física).';
  }
  if (biz?.activity && biz.activity.length > 255) errors.activity = 'Usa un máximo de 255 caracteres.';
  if (biz?.taxRegime && biz.taxRegime.length > 255) errors.taxRegime = 'Usa un máximo de 255 caracteres.';
  if (biz?.fiscalAddress && biz.fiscalAddress.length > 255) errors.fiscalAddress = 'Usa un máximo de 255 caracteres.';

  return errors;
}

/** Display title used across selectors, charge lists, and customer cards. */
export function customerDisplayLabel(record: Pick<CustomerProfileRecord, 'entityType' | 'name' | 'business'>): string {
  if (record.entityType === 'company' && record.business?.legalName) {
    return `${record.business.legalName} (${record.name})`;
  }
  return record.business?.tradeName ? `${record.name} · ${record.business.tradeName}` : record.name;
}

export function canEdit(charge: DashboardChargeDto, field: ChargeField): boolean {
  return charge.editableFields.includes(field);
}

export function formatMxnCents(amountCents: number): string {
  return new Intl.NumberFormat('es-MX', { style: 'currency', currency: 'MXN' }).format(amountCents / 100);
}

/** Pesos typed by the user → integer cents, or null when invalid. */
export function pesosToCents(value: string): number | null {
  const pesos = Number(value);
  if (!Number.isFinite(pesos)) return null;
  const cents = Math.round(pesos * 100);
  return cents >= AMOUNT_MIN_CENTS && cents <= AMOUNT_MAX_CENTS ? cents : null;
}

/** Formats a contract date (YYYY-MM-DD) without shifting it across timezones. */
export function formatDate(date: string): string {
  const [year, month, day] = date.split('-').map(Number);
  return new Intl.DateTimeFormat('es-MX', { day: '2-digit', month: 'short', timeZone: 'UTC' }).format(
    new Date(Date.UTC(year!, month! - 1, day!)),
  );
}

export function formatInstant(instant: string): string {
  return new Intl.DateTimeFormat('es-MX', {
    day: '2-digit',
    month: 'short',
    hour: '2-digit',
    minute: '2-digit',
    timeZone: DASHBOARD_TIMEZONE,
  }).format(new Date(instant));
}

/** Today as YYYY-MM-DD in the contract timezone. */
export function todayInMexico(now: Date = new Date()): string {
  return new Intl.DateTimeFormat('en-CA', { timeZone: DASHBOARD_TIMEZONE }).format(now);
}

export function parseTags(value: string): string[] {
  return [...new Set(value.split(',').map((tag) => tag.trim()).filter(Boolean))];
}
