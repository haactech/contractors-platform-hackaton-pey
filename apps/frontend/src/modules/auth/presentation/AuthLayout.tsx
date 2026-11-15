import type { ReactNode } from 'react';

const HIGHLIGHTS = [
  'Links de pago y QR CoDi sin comisiones ocultas',
  'Cada cobro llega directo a tu CLABE bancaria',
  'Recordatorios y control de clientes en un solo lugar',
] as const;

export function AuthLayout({ children }: { readonly children: ReactNode }) {
  return (
    <section className="auth-shell" aria-label="Acceso a Praxis">
      <aside className="auth-brand-panel" aria-label="Propuesta de valor de Praxis">
        <div className="auth-brand-top">
          <div className="auth-brand-mark">
            <span className="auth-logo-badge" aria-hidden="true">P</span>
            <div>
              <strong className="auth-brand-name">Praxis</strong>
              <span className="auth-brand-tag">Cobros para contratistas</span>
            </div>
          </div>
          <p className="auth-brand-headline">Cobra tu trabajo sin comisiones y sin perseguir comprobantes.</p>
        </div>

        <ul className="auth-highlight-list">
          {HIGHLIGHTS.map((item) => (
            <li key={item}>
              <span className="auth-check-dot" aria-hidden="true">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.8">
                  <path d="M5 13l4 4L19 7" strokeLinecap="round" strokeLinejoin="round" />
                </svg>
              </span>
              <span>{item}</span>
            </li>
          ))}
        </ul>

        <p className="auth-brand-footnote">
          Si es tu primera vez, después de crear tu cuenta completarás los datos de tu negocio y tu CLABE en 3 pasos cortos.
        </p>
      </aside>

      <div className="auth-card">
        <div className="auth-mobile-brand">
          <span className="auth-logo-badge" aria-hidden="true">P</span>
          <strong>Praxis</strong>
        </div>
        {children}
      </div>
    </section>
  );
}
