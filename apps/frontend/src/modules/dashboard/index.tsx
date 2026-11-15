import type { DashboardGateway, PaymentsGateway } from './application/dashboard-gateway';
import { httpCustomersDirectory } from './infrastructure/http-clients-gateway';
import { httpDashboardGateway } from './infrastructure/http-dashboard-gateway';
import { httpPaymentsGateway } from './infrastructure/http-payments-gateway';
import { createMockDashboard } from './infrastructure/mock-dashboard-gateway';
import { DashboardWorkspace as Workspace } from './presentation/DashboardWorkspace';

const mock = createMockDashboard();

/**
 * Profile, clients, and charge creation talk to the API. The charge list,
 * reminders, and summary stay on the local adapter until those routes exist.
 */
const gateway: DashboardGateway = { ...mock, profile: httpDashboardGateway.profile };
const payments: PaymentsGateway = { ...mock, createCharge: httpPaymentsGateway.createCharge };

interface DashboardWorkspaceProps {
  readonly contractorId: string;
  /** Called when the backend does not know the contractor (404). */
  readonly onNotFound: () => void;
  readonly onSignOut: () => void;
}

export function DashboardWorkspace({ contractorId, onNotFound, onSignOut }: DashboardWorkspaceProps) {
  return (
    <Workspace
      gateway={gateway}
      payments={payments}
      customersDirectory={httpCustomersDirectory}
      demo={mock}
      contractorId={contractorId}
      onNotFound={onNotFound}
      onSignOut={onSignOut}
    />
  );
}
