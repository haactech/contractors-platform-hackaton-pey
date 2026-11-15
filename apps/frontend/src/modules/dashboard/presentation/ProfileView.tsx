import { useId, useState, type FormEvent } from 'react';
import type {
  DashboardProfileDto,
  OnboardingField,
  UpdateOnboardingProfileRequest,
} from '@platform/contracts';
import {
  BUSINESS_ACTIVITIES,
  OTHER_BUSINESS_ACTIVITY,
  RFC_PATTERN,
  isValidClabe,
  normalizeClabe,
} from '../../../shared/business';
import { formatMxPhone } from '../domain/dashboard';

export interface DemoControls {
  simulateProviderPayment(contractorId?: string): string | null;
  simulateConcurrentEdit(chargeId: string, contractorId?: string): void;
  reset(contractorId?: string): void;
  loadDemoData(contractorId?: string): void;
}

interface ProfileViewProps {
  readonly profile: DashboardProfileDto;
  readonly clabeLabel: string;
  readonly demo?: DemoControls;
  readonly onUpdateProfile?: (patch: UpdateOnboardingProfileRequest) => Promise<void>;
  readonly onSimulatePayment: () => void;
  readonly onSimulateConflict: () => void;
  readonly onResetDemo: () => void;
  readonly onLoadDemo: () => void;
  readonly onSignOut?: () => void;
}

const TAXPAYER_LABELS: Record<DashboardProfileDto['business']['taxpayerType'], string> = {
  individual: 'Persona física',
  company: 'Persona moral',
  unknown: 'Por confirmar',
};

const PENDING_FIELD_LABELS: Record<OnboardingField, string> = {
  businessName: 'Nombre del negocio',
  businessActivity: 'Giro',
  rfc: 'RFC',
  taxRegime: 'Régimen fiscal',
  clabe: 'CLABE',
};

const KNOWN_ACTIVITIES: readonly string[] = BUSINESS_ACTIVITIES;

function initialActivitySelection(activity: string): { select: string; other: string } {
  if (!activity) return { select: '', other: '' };
  if (KNOWN_ACTIVITIES.includes(activity)) return { select: activity, other: '' };
  return { select: OTHER_BUSINESS_ACTIVITY, other: activity };
}

function validateProfileForm(fields: {
  name: string;
  profession: string;
  businessName: string;
  businessActivity: string;
  rfc: string;
  taxRegime: string;
  clabeDigits: string;
  requireClabe: boolean;
}): Record<string, string> {
  const errors: Record<string, string> = {};
  if (!fields.name.trim()) errors.name = 'Este campo es obligatorio.';
  else if (fields.name.trim().length > 255) errors.name = 'Usa un máximo de 255 caracteres.';

  if (!fields.profession.trim()) errors.profession = 'Este campo es obligatorio.';
  else if (fields.profession.trim().length > 255) errors.profession = 'Usa un máximo de 255 caracteres.';

  if (fields.businessName.trim().length > 255) errors.businessName = 'Usa un máximo de 255 caracteres.';
  if (fields.businessActivity.trim().length > 255) errors.businessActivity = 'Usa un máximo de 255 caracteres.';
  if (fields.taxRegime.trim().length > 255) errors.taxRegime = 'Usa un máximo de 255 caracteres.';

  const normalizedRfc = fields.rfc.trim().toUpperCase();
  if (normalizedRfc && !RFC_PATTERN.test(normalizedRfc)) {
    errors.rfc = 'Escribe un RFC de 12 o 13 caracteres con formato válido.';
  }
  if (fields.requireClabe && !fields.clabeDigits) {
    errors.clabe = 'Este campo es obligatorio.';
  } else if (fields.clabeDigits && !isValidClabe(fields.clabeDigits)) {
    errors.clabe = 'Escribe una CLABE válida de 18 dígitos.';
  }
  return errors;
}

export function ProfileView({
  profile,
  clabeLabel,
  demo,
  onUpdateProfile,
  onSimulatePayment,
  onSimulateConflict,
  onResetDemo,
  onLoadDemo,
  onSignOut,
}: ProfileViewProps) {
  const baseId = useId();
  const [editing, setEditing] = useState(false);
  const [saving, setSaving] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);
  const [errors, setErrors] = useState<Record<string, string>>({});

  const [name, setName] = useState(profile.name);
  const [profession, setProfession] = useState(profile.profession);
  const [businessName, setBusinessName] = useState(profile.business.name);
  const initialActivity = initialActivitySelection(profile.business.activity);
  const [activitySelect, setActivitySelect] = useState(initialActivity.select);
  const [activityOther, setActivityOther] = useState(initialActivity.other);
  const [rfc, setRfc] = useState(profile.business.rfc);
  const [taxRegime, setTaxRegime] = useState(profile.business.taxRegime ?? '');
  const [clabeInput, setClabeInput] = useState('');

  function openEditor() {
    const nextActivity = initialActivitySelection(profile.business.activity);
    setName(profile.name);
    setProfession(profile.profession);
    setBusinessName(profile.business.name);
    setActivitySelect(nextActivity.select);
    setActivityOther(nextActivity.other);
    setRfc(profile.business.rfc);
    setTaxRegime(profile.business.taxRegime ?? '');
    setClabeInput('');
    setErrors({});
    setFormError(null);
    setEditing(true);
  }

  function clearFieldError(field: string) {
    setErrors((prev) => {
      if (!prev[field]) return prev;
      const next = { ...prev };
      delete next[field];
      return next;
    });
    setFormError(null);
  }

  async function handleSave(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!onUpdateProfile || saving) return;

    const resolvedActivity =
      activitySelect === OTHER_BUSINESS_ACTIVITY ? activityOther.trim() : activitySelect.trim();
    const clabeDigits = normalizeClabe(clabeInput);

    const validation = validateProfileForm({
      name,
      profession,
      businessName,
      businessActivity: resolvedActivity,
      rfc,
      taxRegime,
      clabeDigits,
      requireClabe: !profile.business.clabeLast4,
    });
    setErrors(validation);
    setFormError(null);
    if (Object.keys(validation).length > 0) {
      return;
    }

    const patch: UpdateOnboardingProfileRequest = {
      name: name.trim(),
      profession: profession.trim(),
      businessName: businessName.trim(),
      businessActivity: resolvedActivity,
      rfc: rfc.trim().toUpperCase(),
      taxRegime: taxRegime.trim(),
    };
    if (clabeDigits) {
      patch.clabe = clabeDigits;
    }

    setSaving(true);
    try {
      await onUpdateProfile(patch);
      setEditing(false);
      setClabeInput('');
    } catch (error) {
      const apiErr = error as { message?: string; fields?: Record<string, string> };
      if (apiErr.fields && Object.keys(apiErr.fields).length > 0) {
        setErrors(apiErr.fields);
      }
      setFormError(apiErr.message ?? 'No fue posible actualizar tu perfil.');
    } finally {
      setSaving(false);
    }
  }

  const pendingLabels = profile.onboarding.pendingFields.map(
    (field) => PENDING_FIELD_LABELS[field] ?? field,
  );
  const rfcSummary = profile.business.rfc
    ? `${profile.business.rfc} · ${profile.business.taxRegime ?? 'Por definir'} (${TAXPAYER_LABELS[profile.business.taxpayerType]})`
    : profile.business.taxRegime ?? 'Por definir';

  return (
    <div className="db-stack">
      <div className="db-section-header">
        <div>
          <h2>Perfil y cuenta de depósito</h2>
          <p className="db-muted">Datos del titular, cuenta CLABE y datos opcionales de negocio o régimen fiscal.</p>
        </div>
        <div className="db-row-actions">
          {onUpdateProfile && !editing && (
            <button type="button" className="db-btn-primary" onClick={openEditor}>
              Editar perfil
            </button>
          )}
          {onSignOut && (
            <button type="button" className="db-btn-secondary" onClick={onSignOut}>
              Cerrar sesión
            </button>
          )}
        </div>
      </div>

      {profile.onboarding.status === 'incomplete' && !editing && onUpdateProfile && (
        <div className="db-company-box" role="status">
          <div className="db-section-header">
            <div>
              <p className="db-company-box-title">Datos de negocio y cobro pendientes</p>
              <p className="db-muted">
                Omitiste algunos datos opcionales en el alta ({pendingLabels.join(', ')}). Puedes agregarlos cuando los tengas.
              </p>
            </div>
            <button type="button" className="db-btn-accent" onClick={openEditor}>
              Completar ahora
            </button>
          </div>
        </div>
      )}

      {editing ? (
        <section className="db-card db-stack" aria-label="Editar perfil del contratista">
          <div className="db-section-header">
            <div>
              <h3>Editar perfil y datos fiscales</h3>
              <p className="db-muted">
                Nombre, profesión y CLABE son obligatorios; los datos del negocio y RFC siguen siendo opcionales.
              </p>
            </div>
          </div>

          <form className="db-stack" onSubmit={(event) => void handleSave(event)} noValidate>
            <div className="db-form-grid">
              <div className="form-field">
                <label htmlFor={`${baseId}-name`}>Nombre completo *</label>
                <input
                  id={`${baseId}-name`}
                  name="name"
                  value={name}
                  disabled={saving}
                  aria-required="true"
                  aria-invalid={Boolean(errors.name)}
                  onChange={(event) => {
                    setName(event.target.value);
                    clearFieldError('name');
                  }}
                />
                {errors.name && (
                  <small role="alert" className="field-error">
                    {errors.name}
                  </small>
                )}
              </div>

              <div className="form-field">
                <label htmlFor={`${baseId}-profession`}>Profesión u oficio *</label>
                <input
                  id={`${baseId}-profession`}
                  name="profession"
                  value={profession}
                  disabled={saving}
                  aria-required="true"
                  aria-invalid={Boolean(errors.profession)}
                  onChange={(event) => {
                    setProfession(event.target.value);
                    clearFieldError('profession');
                  }}
                />
                {errors.profession && (
                  <small role="alert" className="field-error">
                    {errors.profession}
                  </small>
                )}
              </div>

              <div className="form-field">
                <label htmlFor={`${baseId}-businessName`}>Nombre del negocio (opcional)</label>
                <input
                  id={`${baseId}-businessName`}
                  name="businessName"
                  placeholder="Ej. Servicios Eléctricos García"
                  value={businessName}
                  disabled={saving}
                  aria-invalid={Boolean(errors.businessName)}
                  onChange={(event) => {
                    setBusinessName(event.target.value);
                    clearFieldError('businessName');
                  }}
                />
                {errors.businessName && (
                  <small role="alert" className="field-error">
                    {errors.businessName}
                  </small>
                )}
              </div>

              <div className="form-field">
                <label htmlFor={`${baseId}-businessActivity`}>Giro del negocio (opcional)</label>
                <select
                  id={`${baseId}-businessActivity`}
                  name="businessActivity"
                  value={activitySelect}
                  disabled={saving}
                  aria-invalid={Boolean(errors.businessActivity)}
                  onChange={(event) => {
                    setActivitySelect(event.target.value);
                    if (event.target.value !== OTHER_BUSINESS_ACTIVITY) {
                      setActivityOther('');
                    }
                    clearFieldError('businessActivity');
                  }}
                >
                  <option value="">Sin especificar</option>
                  {BUSINESS_ACTIVITIES.map((option) => (
                    <option key={option} value={option}>
                      {option}
                    </option>
                  ))}
                  <option value={OTHER_BUSINESS_ACTIVITY}>{OTHER_BUSINESS_ACTIVITY}</option>
                </select>
                {activitySelect === OTHER_BUSINESS_ACTIVITY && (
                  <input
                    name="businessActivityOther"
                    placeholder="Escribe el giro del negocio"
                    value={activityOther}
                    disabled={saving}
                    onChange={(event) => {
                      setActivityOther(event.target.value);
                      clearFieldError('businessActivity');
                    }}
                  />
                )}
                {errors.businessActivity && (
                  <small role="alert" className="field-error">
                    {errors.businessActivity}
                  </small>
                )}
              </div>

              <div className="form-field">
                <label htmlFor={`${baseId}-rfc`}>RFC (opcional)</label>
                <input
                  id={`${baseId}-rfc`}
                  name="rfc"
                  maxLength={13}
                  placeholder="Ej. GALA900101XXX"
                  value={rfc}
                  disabled={saving}
                  aria-invalid={Boolean(errors.rfc)}
                  onChange={(event) => {
                    setRfc(event.target.value.toUpperCase());
                    clearFieldError('rfc');
                  }}
                />
                <small className="db-muted">12 caracteres (persona moral) o 13 (persona física).</small>
                {errors.rfc && (
                  <small role="alert" className="field-error">
                    {errors.rfc}
                  </small>
                )}
              </div>

              <div className="form-field">
                <label htmlFor={`${baseId}-taxRegime`}>Régimen fiscal (opcional)</label>
                <input
                  id={`${baseId}-taxRegime`}
                  name="taxRegime"
                  placeholder="Ej. RESICO"
                  value={taxRegime}
                  disabled={saving}
                  aria-invalid={Boolean(errors.taxRegime)}
                  onChange={(event) => {
                    setTaxRegime(event.target.value);
                    clearFieldError('taxRegime');
                  }}
                />
                {errors.taxRegime && (
                  <small role="alert" className="field-error">
                    {errors.taxRegime}
                  </small>
                )}
              </div>

              <div className="form-field db-field-span-2">
                <label htmlFor={`${baseId}-clabe`}>
                  CLABE para recibir tus pagos{!profile.business.clabeLast4 ? ' *' : ''}
                </label>
                <input
                  id={`${baseId}-clabe`}
                  name="clabe"
                  inputMode="numeric"
                  maxLength={22}
                  placeholder={
                    profile.business.clabeLast4
                      ? `CLABE actual •••• ${profile.business.clabeLast4} (escribe 18 dígitos para cambiarla)`
                      : '18 dígitos de tu cuenta bancaria'
                  }
                  value={clabeInput}
                  disabled={saving}
                  aria-required={!profile.business.clabeLast4}
                  aria-invalid={Boolean(errors.clabe)}
                  onChange={(event) => {
                    setClabeInput(event.target.value.replace(/[^\d\s-]/g, ''));
                    clearFieldError('clabe');
                  }}
                />
                {profile.business.clabeLast4 && (
                  <small className="db-muted">
                    Deja este campo vacío para conservar tu CLABE actual (•••• {profile.business.clabeLast4}), o escribe 18 dígitos nuevos para actualizarla.
                  </small>
                )}
                {errors.clabe && (
                  <small role="alert" className="field-error">
                    {errors.clabe}
                  </small>
                )}
              </div>
            </div>

            {formError && (
              <p role="alert" className="db-error-banner">
                {formError}
              </p>
            )}

            <div className="db-row-actions">
              <button
                type="button"
                className="db-btn-secondary"
                disabled={saving}
                onClick={() => setEditing(false)}
              >
                Cancelar
              </button>
              <button type="submit" className="db-btn-accent" disabled={saving}>
                {saving ? 'Guardando cambios...' : 'Guardar cambios'}
              </button>
            </div>
          </form>
        </section>
      ) : (
        <section className="db-card db-stack" aria-label="Datos del negocio">
          <div className="db-profile-grid">
            <div className="db-profile-item">
              <span>Titular</span>
              <strong>{profile.name}</strong>
            </div>
            <div className="db-profile-item">
              <span>Negocio</span>
              <strong>{profile.business.name || 'No registrado'}</strong>
            </div>
            <div className="db-profile-item">
              <span>Profesión</span>
              <strong>{profile.profession}</strong>
            </div>
            <div className="db-profile-item">
              <span>Giro</span>
              <strong>{profile.business.activity || 'No especificado'}</strong>
            </div>
            <div className="db-profile-item">
              <span>RFC y régimen</span>
              <strong>{rfcSummary}</strong>
            </div>
            <div className="db-profile-item">
              <span>Cuenta CLABE destino</span>
              <strong className="tabular">{clabeLabel}</strong>
            </div>
            <div className="db-profile-item">
              <span>Primer cliente registrado</span>
              <strong>
                {profile.firstCustomer.name} · +52 {formatMxPhone(profile.firstCustomer.phone)}
              </strong>
            </div>
            <div className="db-profile-item">
              <span>Estado del onboarding</span>
              <strong>
                {profile.onboarding.status === 'complete'
                  ? 'Completo'
                  : `Pendiente: ${pendingLabels.join(', ')}`}
              </strong>
            </div>
          </div>
        </section>
      )}

      {demo && (
        <section className="db-card db-stack" aria-label="Herramientas de prueba local">
          <div>
            <h3>Herramientas de prueba local</h3>
            <p className="db-muted">
              Simula notificaciones CoDi o conflictos de concurrencia mientras el módulo de pagos backend está en desarrollo.
            </p>
          </div>
          <div className="db-row-actions">
            <button type="button" className="db-btn-primary" onClick={onSimulatePayment}>
              Simular pago CoDi recibido
            </button>
            <button type="button" className="db-btn-secondary" onClick={onSimulateConflict}>
              Simular conflicto v409
            </button>
            <button type="button" className="db-btn-secondary" onClick={onLoadDemo}>
              Cargar 5 cobros de prueba
            </button>
            <button type="button" className="db-btn-ghost" onClick={onResetDemo}>
              Vaciar datos (ver el primer cobro)
            </button>
          </div>
        </section>
      )}
    </div>
  );
}
