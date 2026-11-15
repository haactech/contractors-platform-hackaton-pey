import { Container, getContainer } from '@cloudflare/containers';

const FORWARDED_ENV_KEYS = [
  'PEY_API_KEY',
  'PEY_API_URL',
  'PEY_WEBHOOK_SECRET',
  'PAYMENT_SUCCESS_URL',
  'PAYMENT_BACK_URL',
  'AUTH_MODE',
  'AUTH_REQUIRE_EMAIL_VERIFICATION',
  'AUTH_LOCKOUT',
  'AUTH_PIN_RESET_REQUIRES_CODE',
  'AUTH_DEMO_ACCOUNT',
  'AUTH_EXPOSE_CODES',
  'SMTP_HOST',
  'SMTP_PORT',
  'SMTP_USERNAME',
  'SMTP_PASSWORD',
  'MAIL_FROM',
] as const;

type ForwardedEnvKey = (typeof FORWARDED_ENV_KEYS)[number];

interface Env extends Partial<Record<ForwardedEnvKey, string>> {
  ASSETS: Fetcher;
  BACKEND: DurableObjectNamespace<Backend>;
}

function selectContainerEnv(env: Env): Record<string, string> {
  const vars: Record<string, string> = {};
  for (const key of FORWARDED_ENV_KEYS) {
    const value = env[key];
    if (typeof value === 'string' && value.length > 0) {
      vars[key] = value;
    }
  }
  return vars;
}

export class Backend extends Container<Env> {
  defaultPort = 8080;
  sleepAfter = '15m';

  constructor(ctx: ConstructorParameters<typeof Container<Env>>[0], env: Env) {
    super(ctx, env);
    this.envVars = selectContainerEnv(env);
  }
}

export default {
  async fetch(request: Request, env: Env): Promise<Response> {
    const { pathname } = new URL(request.url);
    if (pathname === '/api' || pathname.startsWith('/api/')) {
      // Route requests to the same container while the repository is in memory.
      return getContainer(env.BACKEND).fetch(request);
    }
    return env.ASSETS.fetch(request);
  },
} satisfies ExportedHandler<Env>;
