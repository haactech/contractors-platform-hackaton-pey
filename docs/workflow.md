# Flujo funcional y módulos de pagos

Este documento define el flujo completo de la plataforma en tres etapas (`Stage 1`, `Stage 2` y `Stage 3`), alineado con la [arquitectura modular](architecture.md) del monorepo (`apps/backend`, `apps/frontend` y `packages/contracts`).

En esta iteración se soportan tres modalidades de cobro en `api.pey.mx`:
- **`PAYMENT_LINK`** (`POST /payment-link/create`): enlace de cobro alojado para enviar por WhatsApp, correo electrónico o copiar enlace.
- **`CODI_QR`** (`POST /codi/qr`): código QR CoDi dinámico para cobro presencial desde pantalla.
- **`CODI_PUSH`** (`POST /codi/pn`): push CoDi al teléfono de 10 dígitos del cliente guardado (sin QR ni enlace).

---

## 1. Diagrama general de flujo (Stages 1, 2 y 3)

```mermaid
flowchart TD
    subgraph S1["STAGE 1 · Welcome & Onboarding (implementado en /api/onboarding)"]
        W["Welcome Screen\nPropuesta de valor (0% comisión CoDi)"] -->         OB_FORM["Formulario de Onboarding:\n• Nombre, Profesión y CLABE\n• Negocio, Giro y RFC / Régimen Fiscal\n• 1.er Cliente (Nombre, Teléfono, Email)"]
        OB_FORM --> VALID{"¿Validación HTTP 422 / 201?"}
        VALID -- "Error 422" --> ERR["Muestra errores por campo en español"]
        ERR --> OB_FORM
        VALID -- "201 Created" --> PEY_BA["Pey POST /bank-account (soft-fail OK)\nRespuesta: clabeLast4 + codiStatus + bankReady"]
        PEY_BA --> READY{"¿bankReady?"}
        READY -- "No" --> RETRY["POST …/bank-account/retry\no espera ACTIVE"]
        RETRY --> READY
        READY -- "Sí" --> FIRST_CHARGE["Guía en Dashboard: Haz tu 1.er cobro"]
    end

    subgraph S2["STAGE 2 · Creación y entrega del cobro (Payments)"]
        FIRST_CHARGE --> REQ["Solicitar pago (Request Payment)\nSelecciona Cliente + Concepto + Monto (centavos)"]
        DASH_CTA["Acción rápida 'Nuevo cobro' o Asistente"] --> REQ
        REQ --> METHOD{"Método de cobro\n(paymentMethod)"}

        METHOD -- "PAYMENT_LINK" --> LNK["POST /payment-link/create\nGenera URL de pago en pey"]
        LNK --> LNK_SEND["Compartir enlace por:\n• WhatsApp (wa.me)\n• Correo electrónico\n• Copiar link"]

        METHOD -- "CODI_QR" --> QR["POST /codi/qr\nGenera QR CoDi dinámico"]
        QR --> QR_UI["Muestra QR en pantalla\npara escaneo presencial"]

        METHOD -- "CODI_PUSH" --> PN["POST /codi/pn\nPush al teléfono 10 dígitos del cliente"]
        PN --> PN_OK["Cobro PENDING sin link/QR\n(requiere teléfono MX usable)"]
    end

    subgraph S3["STAGE 3 · Dashboard, conciliación y recordatorios"]
        LNK_SEND --> PENDING["Cobro en estado PENDING_PAYMENT\n• Fila en tabla con badge 'Pendiente'\n• Suma al saldo pendiente del cliente (Per Client Balance)"]
        QR_UI --> PENDING
        PN_OK --> PENDING

        PENDING --> PAID{"¿Pago completado?"}
        PAID -- "Webhook firmado de pey" --> WH["POST /api/webhooks/pey\n(standardwebhooks)"]
        WH --> SETTLED["Actualiza estado a SETTLED ('Pagado')\n• Incrementa total cobrado\n• Descuenta saldo pendiente del cliente"]

        PAID -- "Sigue pendiente" --> REM["Acción en tabla: Recordar (MSJ Reminder)\nReenvía enlace por WhatsApp o correo"]
        REM --> PENDING
    end
```

---

## 2. Secuencia entre Frontend, Backend Go y `api.pey.mx`

```mermaid
sequenceDiagram
    autonumber
    actor P as Contratista / Profesional
    participant Web as Frontend (React + Vite)
    participant API as Backend (Go net/http)
    participant Pey as api.pey.mx (Payment Link / CoDi QR / CoDi Push)
    actor C as Cliente

    Note over P,C: Flujo A: Creación del cobro (Payment Link, CoDi QR o CoDi Push)
    P->>Web: Selecciona cliente, concepto y monto; elige PAYMENT_LINK, CODI_QR o CODI_PUSH
    Web->>API: POST /api/payments (CreatePaymentIntentRequest)
    API->>Pey: POST /payment-link/create, /codi/qr o /codi/pn (monto en pesos)
    Pey-->>API: Devuelve paymentLinkUrl, qrString+id, o transactionId
    API-->>Web: 201 PaymentIntentDto (status = PENDING_PAYMENT)
    Web->>C: Comparte link, presenta QR, o el cliente recibe el push CoDi
    Note over API,Pey: CODI_PUSH exige bankReady + teléfono guardado reducible a 10 dígitos MX

    Note over P,C: Flujo B: Recordatorio de pago (MSJ Reminder)
    P->>Web: Filtra por "Pendientes" y pulsa "Recordar"
    Web->>API: POST /api/payments/{id}/remind
    API-->>Web: Actualiza contador de recordatorios y devuelve enlace/mensaje
    Web->>C: Abre WhatsApp o correo con el recordatorio y el Payment Link

    Note over P,C: Flujo C: Conciliación vía webhook
    C->>Pey: Realiza el pago desde el enlace o escaneando el QR CoDi
    Pey->>API: POST /api/webhooks/pey (firma standardwebhooks)
    API->>API: Verifica firma + idempotencia (webhook-id) + cambia estado a SETTLED
    API-->>Web: Dashboard refleja badge "Pagado" y actualiza saldo del cliente
```

---

## 3. Máquina de estados del cobro (`PaymentStatusDto`)

```mermaid
stateDiagram-v2
    [*] --> PENDING_PAYMENT: Crea cobro (PAYMENT_LINK, CODI_QR o CODI_PUSH)
    PENDING_PAYMENT --> PENDING_PAYMENT: Envía recordatorio (MSJ Reminder)
    PENDING_PAYMENT --> SETTLED: Webhook pey (monto íntegro)
    PENDING_PAYMENT --> UNDERPAID: Webhook pey (monto parcial)
    PENDING_PAYMENT --> EXPIRED: Expira vigencia sin pago
    PENDING_PAYMENT --> CANCELLED: Cancelado por el contratista
    EXPIRED --> SETTLED: Webhook de pago tardío
    CANCELLED --> SETTLED: Webhook de pago tardío
```

### Reglas de interfaz en el Dashboard (`Stage 3`)
1. **Badges semánticos (texto + color):**
   - `SETTLED` → `Pagado`
   - `PENDING_PAYMENT` → `Pendiente`
   - `EXPIRED` → `Vencido`
   - `UNDERPAID` → `Incompleto`
   - `CANCELLED` → `Cancelado`
2. **Filtros de tabla:** `Todos`, `Pendientes`, `Completados` y `Vencidos`.
3. **Balance por cliente (`Per Client Balance`):** Calcula por cada cliente el monto total pagado (`settledAmountCents`) y el saldo pendiente (`pendingAmountCents`) a partir de sus cobros registrados.
4. **Representación monetaria:** Todos los montos en contratos e interfaz interna viajan en centavos enteros (`amountCents`) y se formatean con números tabulares (`font-variant-numeric: tabular-nums`).
