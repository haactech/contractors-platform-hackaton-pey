import type { ApiErrorResponse, ChargeField, CreatePaymentIntentRequest, PaymentIntentDto } from '@platform/contracts';
import { DashboardApiError, type CreatedChargeResult, type NewChargeInput, type PaymentsGateway } from '../application/dashboard-gateway';
import { todayInMexico } from '../domain/dashboard';

const PENDING_FIELDS: ChargeField[] = ['concept', 'dueDate', 'amountCents', 'note', 'tags'];

function toCharge(intent: PaymentIntentDto, customerId: string, dueDate: string): CreatedChargeResult['charge'] {
  const storedDueDate = intent.dueDate ?? dueDate;
  return {
    chargeId: intent.id,
    version: 1,
    customerId,
    customerName: intent.clientName,
    concept: intent.concept,
    amountCents: intent.amountCents,
    dueDate: storedDueDate,
    status: 'pending',
    overdue: storedDueDate !== '' && storedDueDate < todayInMexico(),
    createdAt: intent.createdAt,
    updatedAt: intent.createdAt,
    paidAt: null,
    paymentMethod: null,
    cancelledAt: null,
    reminderCount: intent.reminderCount,
    note: null,
    tags: intent.method === 'PAYMENT_LINK' ? ['link'] : ['codi'],
    editableFields: PENDING_FIELDS,
  };
}

/** Posts the screen's amount and client to POST /api/payments. The bank id stays on the server. */
export const httpPaymentsGateway: Pick<PaymentsGateway, 'createCharge'> = {
  async createCharge(contractorId, input: NewChargeInput): Promise<CreatedChargeResult> {
    const body: CreatePaymentIntentRequest = {
      onboardingId: contractorId,
      clientId: input.customerId,
      clientName: input.clientName?.trim() ?? '',
      concept: input.concept,
      amountCents: input.amountCents,
      dueDate: input.dueDate,
      method: input.method,
    };
    const phone = input.clientPhone?.trim();
    const email = input.clientEmail?.trim();
    if (phone) body.clientPhone = phone;
    if (email) body.clientEmail = email;

    const response = await fetch('/api/payments', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
      body: JSON.stringify(body),
    });
    if (!response.ok) {
      let error: ApiErrorResponse | undefined;
      try {
        error = await response.json();
      } catch {
        /* Non-JSON error body. */
      }
      const fieldMessage = error?.errors ? Object.values(error.errors).find((message) => message) : undefined;
      throw new DashboardApiError(fieldMessage ?? error?.message ?? 'No fue posible crear el cobro.', response.status, error?.code, error?.errors);
    }
    const intent: PaymentIntentDto = await response.json();
    return {
      charge: toCharge(intent, intent.clientId ?? input.customerId, input.dueDate),
      method: intent.method,
      ...(intent.paymentLinkUrl ? { paymentLinkUrl: intent.paymentLinkUrl } : {}),
      ...(intent.codiQrString ? { codiQrString: intent.codiQrString } : {}),
      ...(intent.codiQrImage ? { codiQrImage: intent.codiQrImage } : {}),
    };
  },
};
