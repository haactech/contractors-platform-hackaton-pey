import type { ApiErrorResponse, CustomerRowDto, VendorClientDto, VendorClientWriteDto } from '@platform/contracts';
import { DashboardApiError, type CustomersDirectoryGateway } from '../application/dashboard-gateway';
import {
  customerDisplayLabel,
  type CustomerProfileRecord,
  type NewCustomerInput,
} from '../domain/dashboard';

function clientBody(input: NewCustomerInput): VendorClientWriteDto {
  return {
    entityType: input.entityType,
    name: input.name,
    phone: input.phone,
    email: input.email,
    ...(input.business?.legalName ? { legalName: input.business.legalName } : {}),
    ...(input.business?.tradeName ? { tradeName: input.business.tradeName } : {}),
    ...(input.business?.rfc ? { rfc: input.business.rfc } : {}),
    ...(input.business?.activity ? { activity: input.business.activity } : {}),
    ...(input.business?.taxRegime ? { taxRegime: input.business.taxRegime } : {}),
    ...(input.business?.fiscalAddress ? { fiscalAddress: input.business.fiscalAddress } : {}),
  };
}

async function readClient(response: Response): Promise<VendorClientDto> {
  if (!response.ok) throw await toError(response);
  return response.json() as Promise<VendorClientDto>;
}

async function toError(response: Response): Promise<DashboardApiError> {
  let error: ApiErrorResponse | undefined;
  try {
    error = await response.json();
  } catch {
    /* Non-JSON error body. */
  }
  const fieldMessage = error?.errors ? Object.values(error.errors).find((message) => message) : undefined;
  return new DashboardApiError(fieldMessage ?? error?.message ?? 'No fue posible guardar el cliente.', response.status, error?.code, error?.errors);
}

/** The list has no stored balances yet, so every row stands up to date at zero. */
export function clientToRow(client: VendorClientDto): CustomerRowDto {
  return {
    customerId: client.clientId,
    name: customerDisplayLabel({
      entityType: client.entityType,
      name: client.name,
      business: businessOf(client),
    }),
    standing: 'up_to_date',
    balance: { outstandingCents: 0, overdueCents: 0 },
    lastPayment: null,
    nextDueDate: null,
    oldestDueDate: null,
  };
}

export function clientToProfile(client: VendorClientDto): CustomerProfileRecord {
  const business = businessOf(client);
  return {
    customerId: client.clientId,
    entityType: client.entityType,
    name: client.name,
    phone: client.phone,
    email: client.email,
    ...(business ? { business } : {}),
  };
}

function businessOf(client: VendorClientDto): CustomerProfileRecord['business'] {
  if (!client.legalName && !client.tradeName && !client.rfc && !client.activity && !client.taxRegime && !client.fiscalAddress) {
    return undefined;
  }
  return {
    legalName: client.legalName ?? '',
    ...(client.tradeName ? { tradeName: client.tradeName } : {}),
    ...(client.rfc ? { rfc: client.rfc } : {}),
    ...(client.activity ? { activity: client.activity } : {}),
    ...(client.taxRegime ? { taxRegime: client.taxRegime } : {}),
    ...(client.fiscalAddress ? { fiscalAddress: client.fiscalAddress } : {}),
  };
}

/** Reads and writes /api/contractors/{contractorId}/clients. */
export const httpCustomersDirectory: CustomersDirectoryGateway = {
  async list(contractorId, signal) {
    const response = await fetch(`/api/contractors/${encodeURIComponent(contractorId)}/clients`, {
      headers: { Accept: 'application/json' },
      signal,
    });
    if (!response.ok) throw await toError(response);
    return response.json() as Promise<VendorClientDto[]>;
  },

  async createCustomer(contractorId, input) {
    const response = await fetch(`/api/contractors/${encodeURIComponent(contractorId)}/clients`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
      body: JSON.stringify(clientBody(input)),
    });
    return readClient(response);
  },

  async updateCustomer(contractorId, customerId, input) {
    const response = await fetch(`/api/contractors/${encodeURIComponent(contractorId)}/clients/${encodeURIComponent(customerId)}`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
      body: JSON.stringify(clientBody(input)),
    });
    return readClient(response);
  },
};
