/**
 * Shared business activities and fiscal/CLABE validators reused between
 * contractor onboarding and profile editing.
 */
export const BUSINESS_ACTIVITIES = [
  'Educación y clases particulares',
  'Deportes y entrenamiento',
  'Salud y bienestar',
  'Belleza y estética',
  'Construcción y remodelación',
  'Electricidad y plomería',
  'Reparación y mantenimiento',
  'Limpieza y servicios del hogar',
  'Alimentos y bebidas',
  'Comercio y abarrotes',
  'Diseño, fotografía y creatividad',
  'Tecnología y soporte',
  'Consultoría y servicios profesionales',
  'Transporte y mensajería',
  'Cuidado de mascotas',
] as const;

export const OTHER_BUSINESS_ACTIVITY = 'Otro';

export const RFC_PATTERN = /^[A-ZÑ&]{3,4}\d{6}[A-Z0-9]{3}$/;

const CLABE_WEIGHTS = [3, 7, 1];

export function normalizeClabe(value: string): string {
  return value.replace(/[\s-]/g, '');
}

/** 18 digits whose last one is the Banxico check digit (weights 3, 7, 1). */
export function isValidClabe(clabe: string): boolean {
  if (!/^\d{18}$/.test(clabe)) return false;
  const sum = [...clabe.slice(0, 17)].reduce(
    (total, digit, index) => total + ((Number(digit) * CLABE_WEIGHTS[index % 3]!) % 10),
    0,
  );
  return Number(clabe[17]) === (10 - (sum % 10)) % 10;
}
