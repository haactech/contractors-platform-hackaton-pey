/** Transport DTOs only. No domain entities or business logic. */
export interface ContractorDto {
  id: string;
  name: string;
  specialty: string;
}

export interface ListContractorsResponse {
  contractors: ContractorDto[];
}

export interface ClientDto {
  name: string;
  phone: string;
  email: string;
}

export interface CreateOnboardingRequest {
  name: string;
  profession: string;
  client: ClientDto;
  taxRegime?: string;
  businessName?: string;
  businessActivity?: string;
  rfc?: string;
  /** 18-digit CLABE where the contractor receives payments. Spaces/hyphens allowed on input. */
  clabe: string;
}

export interface UpdateOnboardingProfileRequest {
  name?: string;
  profession?: string;
  businessName?: string;
  businessActivity?: string;
  rfc?: string;
  taxRegime?: string;
  clabe?: string;
}

export interface OnboardingDto {
  id: string;
  name: string;
  profession: string;
  client: ClientDto;
  taxRegime?: string;
  businessName: string;
  businessActivity: string;
  rfc: string;
  clabe: string;
}

export type PaymentMethodDto = "PAYMENT_LINK" | "CODI_QR" | "CODI_PUSH";

/**
 * Client stored for a vendor. The onboarding nested person stays `ClientDto`.
 * Balances are not part of this record.
 */
export interface VendorClientDto {
  clientId: string;
  vendorId: string;
  entityType: "individual" | "company";
  name: string;
  phone: string;
  email: string;
  legalName?: string;
  tradeName?: string;
  rfc?: string;
  activity?: string;
  taxRegime?: string;
  fiscalAddress?: string;
  createdAt: string;
  updatedAt: string;
}

export type PaymentStatusDto =
  | "PENDING_PAYMENT"
  | "SETTLED"
  | "UNDERPAID"
  | "EXPIRED"
  | "CANCELLED";

export interface CreatePaymentIntentRequest {
  onboardingId: string;
  /** When set, the stored client is used and the request phone is ignored. */
  clientId?: string;
  clientName: string;
  clientPhone?: string;
  clientEmail?: string;
  concept: string;
  amountCents: number;
  /** YYYY-MM-DD. The charge form collects it; readers can ignore it. */
  dueDate?: string;
  method: PaymentMethodDto;
}

export interface PaymentIntentDto {
  id: string;
  onboardingId: string;
  clientName: string;
  clientPhone?: string;
  clientEmail?: string;
  concept: string;
  amountCents: number;
  dueDate?: string;
  method: PaymentMethodDto;
  status: PaymentStatusDto;
  paymentLinkUrl?: string;
  codiQrId?: string;
  codiQrString?: string;
  codiQrImage?: string;
  reminderCount: number;
  lastReminderSentAt?: string;
  createdAt: string;
  settledAt?: string;
}

export interface ClientBalanceDto {
  clientName: string;
  clientPhone?: string;
  clientEmail?: string;
  settledAmountCents: number;
  pendingAmountCents: number;
  pendingCount: number;
}

export type AuthErrorCode =
  | 'unauthenticated'
  | 'email_unverified'
  | 'account_locked'
  | 'demo_protected';

export interface AccountDto {
  id: string;
  email: string;
  phone: string;
  contractorId: string | null;
  emailVerified: boolean;
  demo: boolean;
}

export interface SessionResponse {
  account: AccountDto;
}

export interface DemoAccountInfoDto {
  email: string;
  phone: string;
  pin: string;
}

export interface AuthConfigDto {
  emailVerificationRequired: boolean;
  pinResetRequiresCode: boolean;
  demoAccount: DemoAccountInfoDto | null;
}

export interface SignUpRequest {
  email: string;
  phone: string;
  pin: string;
}

export interface LogInRequest {
  identifier: string;
  pin: string;
}

export interface CodeIssuedResponse {
  devCode?: string;
}

export interface ConfirmEmailRequest {
  code: string;
}

export interface StartPinRecoveryRequest {
  identifier: string;
}

export interface ConfirmPinRecoveryRequest {
  identifier: string;
  code?: string;
  pin: string;
}

export interface ApiErrorResponse<Code extends string = DashboardErrorCode> {
  message: string;
  errors?: Record<string, string>;
  code?: Code;
}

/* ---------- Dashboard: GET /api/contractors/{contractorId}/dashboard/* ----------
 * Amounts are integer MXN cents (40000 = $400.00), ratios 0–1,
 * dates YYYY-MM-DD in America/Mexico_City, instants ISO 8601. */

export type TaxpayerType = 'individual' | 'company' | 'unknown';
export type OnboardingStatus = 'complete' | 'incomplete';
export type OnboardingField = 'businessName' | 'businessActivity' | 'rfc' | 'taxRegime' | 'clabe';
/** codi/card come from pey; cash/transfer/other are recorded by the business. */
export type PaymentMethod = 'codi' | 'card' | 'cash' | 'transfer' | 'other';
export type ManualPaymentMethod = Extract<PaymentMethod, 'cash' | 'transfer' | 'other'>;
export type Plan = 'free' | 'pro';
export type ChargeStatus = 'pending' | 'paid' | 'cancelled';
export type CustomerStanding = 'overdue' | 'pending' | 'up_to_date';
export type CustomerSort = 'overdue' | 'name' | 'last_payment';
export type ChargeSort = 'due_date' | 'created' | 'amount';
/** Fields of UpdateDashboardChargeRequest. pending: all; paid/cancelled: note, tags. */
export type ChargeField = 'concept' | 'dueDate' | 'amountCents' | 'note' | 'tags';
/** `ApiErrorResponse.code` for dashboard errors. Branch on it, never on `message`. */
export type DashboardErrorCode =
  | 'version_conflict'
  | 'field_not_editable'
  | 'charge_not_payable'
  | 'charge_not_cancellable'
  | 'payment_in_progress';
export type Granularity = 'day' | 'week' | 'month';
export type ActivityType =
  | 'payment_received'
  | 'charge_created'
  | 'reminder_sent'
  | 'charge_cancelled'
  | 'charge_updated';
/** SSE event name on /dashboard/events. data: ActivityItemDto. */
export type DashboardEventName = 'dashboard.activity';

export interface PeriodDto {
  from: string;
  to: string;
  timezone: 'America/Mexico_City';
}

/** Common query of /summary and /trend. */
export interface PeriodQuery {
  from?: string;
  to?: string;
}

/* /profile */
export interface DashboardProfileDto {
  contractorId: string;
  name: string;
  profession: string;
  business: {
    name: string;
    activity: string;
    rfc: string;
    taxRegime?: string;
    taxpayerType: TaxpayerType;
    /** Last 4 digits of the deposit CLABE; null when not captured yet. */
    clabeLast4: string | null;
  };
  /** Customer captured in onboarding (`client`): receives the first charge. */
  firstCustomer: { name: string; phone: string; email: string };
  onboarding: { status: OnboardingStatus; pendingFields: OnboardingField[] };
}

/* /summary */
export interface BalanceDto {
  amountCents: number;
  count: number;
  customerCount: number;
}

export interface DashboardSummaryDto {
  contractorId: string;
  period: PeriodDto;
  currency: 'MXN';
  /** Instant the snapshot figures refer to. */
  generatedAt: string;
  /** Period figure. */
  collected: { amountCents: number; count: number; changePercent: number | null };
  /** Snapshot: every unpaid charge, overdue included. */
  outstanding: BalanceDto;
  /** Snapshot: unpaid charges past due. Subset of `outstanding`. */
  overdue: BalanceDto & { oldestDaysOverdue: number | null };
  /** Period figure. */
  collectionRate: { ratio: number | null; paidCount: number; issuedCount: number };
  /** Period figure. */
  averageDaysToPay: number | null;
  /** Period figure. */
  reminders: { sent: number; paidWithoutReminderRatio: number | null };
  /** Period figure: card fees avoided on `codi` payments. */
  commissionSavings: {
    amountCents: number;
    assumption: { feeRate: number; fixedFeeCents: number; vatRate: number };
  };
  /** Period figure: one entry per PaymentMethod, zeros included. */
  paymentMethods: { method: PaymentMethod; amountCents: number; count: number }[];
  /** Snapshot. `limit` is null on unlimited plans. */
  customers: { active: number; plan: Plan; limit: number | null };
  /** Snapshot. At most 5, sorted by amountCents desc. */
  topDebtors: {
    customerId: string;
    name: string;
    amountCents: number;
    overdueCount: number;
    oldestDueDate: string;
  }[];
}

/* /trend */
export interface TrendQuery extends PeriodQuery {
  granularity?: Granularity;
}

export interface TrendBucketDto {
  start: string;
  end: string;
  collectedCents: number;
  paidCount: number;
  issuedCents: number;
  issuedCount: number;
}

export interface DashboardTrendDto {
  contractorId: string;
  period: PeriodDto;
  currency: 'MXN';
  granularity: Granularity;
  /** Contiguous, chronological, zeros included. */
  buckets: TrendBucketDto[];
}

/* Pagination */
export interface PageQuery {
  limit?: number;
  cursor?: string;
}

/* /customers */
export interface CustomersQuery extends PageQuery {
  standing?: CustomerStanding;
  search?: string;
  sort?: CustomerSort;
}

export interface CustomerRowDto {
  customerId: string;
  name: string;
  standing: CustomerStanding;
  balance: { outstandingCents: number; overdueCents: number };
  lastPayment: { amountCents: number; paidAt: string; method: PaymentMethod } | null;
  nextDueDate: string | null;
  oldestDueDate: string | null;
}

export interface DashboardCustomersDto {
  customers: CustomerRowDto[];
  /** Filter tabs. Ignores `standing` and `cursor`, honors `search`. */
  counts: { all: number; overdue: number; pending: number; upToDate: number };
  nextCursor: string | null;
}

/* /customers/{customerId} */
export interface DashboardChargeDto {
  chargeId: string;
  /** Send back on every update (optimistic concurrency). */
  version: number;
  customerId: string;
  customerName: string;
  concept: string;
  amountCents: number;
  dueDate: string;
  status: ChargeStatus;
  overdue: boolean;
  createdAt: string;
  updatedAt: string;
  /** Set when status = paid: show the PAGADO stamp. */
  paidAt: string | null;
  paymentMethod: PaymentMethod | null;
  cancelledAt: string | null;
  reminderCount: number;
  /** Private; never shown to the customer. */
  note: string | null;
  tags: string[];
  /** Fields the PATCH accepts right now. */
  editableFields: ChargeField[];
}

export interface DashboardCustomerDetailDto {
  customer: CustomerRowDto;
  phone: string;
  email?: string;
  totals: { paidCents: number; paidCount: number; averageDaysToPay: number | null };
  /** Newest first, cancelled included. */
  charges: DashboardChargeDto[];
  nextCursor: string | null;
}

/* /charges */
export interface ChargesQuery extends PageQuery, PeriodQuery {
  status?: ChargeStatus;
  overdue?: boolean;
  customerId?: string;
  tag?: string;
  search?: string;
  sort?: ChargeSort;
}

export interface DashboardChargesDto {
  charges: DashboardChargeDto[];
  /** Over every charge matching the filters, not only this page. */
  totals: { count: number; amountCents: number };
  nextCursor: string | null;
}

export interface DashboardChargeDetailDto {
  charge: DashboardChargeDto;
  /** Oldest first. */
  timeline: ActivityItemDto[];
}

/** PATCH /charges/{chargeId}. Absent = unchanged; note null clears; tags replaces. */
export interface UpdateDashboardChargeRequest {
  version: number;
  concept?: string;
  dueDate?: string;
  amountCents?: number;
  note?: string | null;
  tags?: string[];
}

/** POST /charges/{chargeId}/manual-payment */
export interface ManualPaymentRequest {
  version: number;
  method: ManualPaymentMethod;
  /** ISO date-time; default now. */
  paidAt?: string;
  note?: string;
}

/** POST /charges/{chargeId}/cancel */
export interface CancelChargeRequest {
  version: number;
  reason?: string;
}

/* /activity and /events */
export interface ActivityItemDto {
  /** Stable; dedupe items received via stream and list. */
  id: string;
  type: ActivityType;
  occurredAt: string;
  customerId: string;
  customerName: string;
  chargeId: string;
  concept: string;
  amountCents: number;
  /** Only for payment_received. */
  paymentMethod: PaymentMethod | null;
}

export interface DashboardActivityDto {
  items: ActivityItemDto[];
  nextCursor: string | null;
}
