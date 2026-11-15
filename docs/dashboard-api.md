# API del dashboard

Tablero del microempresario mexicano para ver y actualizar los cobros que hace a sus clientes. En el código, el negocio se llama `contractor` y su id es el del registro de onboarding.

Contrato formal: `docs/openapi.json` (tag `dashboard`). DTO: `packages/contracts`. Constantes Go: `apps/backend/internal/modules/dashboard/presentation/contract.go`.

## Endpoints

Base: `/api/contractors/{contractorId}/dashboard`. Todas las rutas implementadas requieren sesión autenticada (`cookieAuth` `praxis_session`) dueña del `contractorId` (`401` sin sesión, `403` `email_unverified` si aplica, `404` si el `contractorId` no pertenece a la cuenta).

### Ver

| Endpoint | Pantalla | Estado |
|---|---|---|
| `GET /profile` | Encabezado: datos del negocio y onboarding pendiente | Implementado |
| `GET /summary?from&to` | Tarjetas de KPIs | Contrato |
| `GET /trend?from&to&granularity` | Gráfica de cobrado contra cobros emitidos | Contrato |
| `GET /customers?standing&search&sort&limit&cursor` | Tabla "quién me debe", con pestañas por estado | Contrato |
| `GET /customers/{customerId}?limit&cursor` | Detalle del cliente y su historial | Contrato |
| `GET /charges?status&overdue&customerId&tag&search&from&to&sort&limit&cursor` | Tabla de cobros, con totales | Contrato |
| `GET /charges/{chargeId}` | Detalle del cobro y su historial de eventos | Contrato |
| `GET /activity?limit&cursor` | Actividad reciente | Contrato |
| `GET /events` (SSE) | Sello PAGADO en vivo | Contrato |

### Actualizar

| Endpoint | Acción | Estado del cobro requerido |
|---|---|---|
| `PATCH /charges/{chargeId}` | Editar concepto, vencimiento o monto | `pending` |
| `PATCH /charges/{chargeId}` | Editar nota y etiquetas | cualquiera |
| `POST /charges/{chargeId}/manual-payment` | Marcar pagado en efectivo, por transferencia u otro | `pending` |
| `POST /charges/{chargeId}/cancel` | Cancelar | `pending` |

Cada cobro trae `editableFields`, así que la interfaz no tiene que repetir estas reglas. Las reglas también están en Go: `domain.EditableFields` / `domain.CanEdit`.

**Concurrencia:** todo cambio envía la `version` del cobro. Si cambió mientras tanto (por ejemplo, el cliente pagó por CoDi mientras se editaba el monto), la respuesta es 409 `version_conflict`: hay que volver a pedir el cobro y reintentar.

**Límite con pey:** el dashboard no llama a pey. Al editar el monto, cancelar o registrar un pago manual, el módulo de cobros actualiza o invalida la solicitud activa en pey. Si hay un pago en proceso, responde 409 `payment_in_progress`.

## Convenciones

- **Montos:** centavos enteros MXN (`40000` = $400.00).
- **Proporciones:** de 0 a 1, o `null` cuando el denominador es 0.
- **Fechas:** `AAAA-MM-DD` en `America/Mexico_City`; los instantes van en ISO 8601. Periodo por defecto: del primer día del mes a hoy (máximo 366 días).
- **Paginación:** `limit` (máximo 100) y `cursor` opaco; la respuesta trae `nextCursor`, que es `null` en la última página.
- **Listas para gráficas:** `paymentMethods` y `trend.buckets` no omiten valores en cero.
- **Errores:** `ApiError` con `message` en español, `errors` por campo en 400/422 y `code` cuando la interfaz debe decidir qué hacer: `version_conflict`, `field_not_editable`, `charge_not_payable`, `charge_not_cancellable`, `payment_in_progress`.
- **Métodos de pago:** `codi` y `card` vienen de pey; `cash`, `transfer` y `other` los registra el negocio.
- **Etiquetas:** texto libre del negocio (máximo 10, de 30 caracteres cada una), normalizadas en minúsculas. La nota es privada y nunca la ve el cliente.

## Tiempo real

`/events` emite `dashboard.activity` con un `ActivityItem`. Con `payment_received`, la interfaz muestra el sello PAGADO y vuelve a pedir `summary`, `trend` y el cliente afectado. Para reconectar se usa `Last-Event-ID`; si SSE no está disponible, se consulta `/activity` periódicamente.

## Qué necesita el dashboard del módulo de cobros

| Necesidad | Detalle |
|---|---|
| Lectura de clientes | `id`, `name`, `phone`, `email?`, `active` |
| Lectura de cobros | `id`, `version`, `customerId`, `concept`, `amountCents`, `dueDate`, `status` (`pending`/`paid`/`cancelled`), `createdAt`, `updatedAt`, `paidAt?`, `paymentMethod?`, `cancelledAt?`, `reminderCount`, `note?`, `tags` |
| Operaciones | editar (con `version`), registrar pago manual, cancelar. Cada una se encarga de sus efectos en pey y devuelve conflicto si hay un pago en proceso |
| Eventos | cobro creado, actualizado, pagado, cancelado y recordatorio enviado (para `/activity`, `/events` y el historial) |
| Plan | `free`/`pro` del negocio |

Los valores de `status`, `paymentMethod` y los tipos de evento deben coincidir con este contrato.
