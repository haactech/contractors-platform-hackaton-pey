import { useEffect, useState, type ReactNode } from 'react';
import QRCode from 'qrcode';
import type { PaymentMethodDto } from '@platform/contracts';
import type { CreatedChargeResult } from '../application/dashboard-gateway';
import { formatMxnCents } from '../domain/dashboard';
import { BoltIcon, CodiQrGraphic, CopyIcon, ShareIcon } from './DashboardIcons';

/** Resolves a displayable QR image URL from Pey's `codiQrImage` or `codiQrString`. */
export function useCodiQrSrc(codiQrImage?: string, codiQrString?: string): string | null {
  const [generatedQrDataUrl, setGeneratedQrDataUrl] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    const qrString = codiQrString?.trim();
    if (codiQrImage || !qrString) {
      setGeneratedQrDataUrl(null);
      return;
    }
    QRCode.toDataURL(qrString, { errorCorrectionLevel: 'M', margin: 2, width: 512 })
      .then((url) => {
        if (!cancelled) setGeneratedQrDataUrl(url);
      })
      .catch(() => {
        if (!cancelled) setGeneratedQrDataUrl(null);
      });
    return () => {
      cancelled = true;
    };
  }, [codiQrImage, codiQrString]);

  return codiQrImage || generatedQrDataUrl || null;
}

interface ChargeDeliveryPanelsProps {
  readonly createdResult: CreatedChargeResult | null;
  readonly method: PaymentMethodDto;
  readonly clabeLabel: string;
  readonly fallbackCustomerName: string;
  readonly previewAmountCents: number;
  readonly concept: string;
  readonly customerPhone?: string;
  readonly onNotify: (message: string) => void;
  readonly extraActions?: ReactNode;
}

/** Shared delivery panels (Payment Link box, CoDi QR image, CoDi Push notice, and share buttons). */
export function ChargeDeliveryPanels({
  createdResult,
  method,
  clabeLabel,
  fallbackCustomerName,
  previewAmountCents,
  concept,
  customerPhone,
  onNotify,
  extraActions,
}: ChargeDeliveryPanelsProps) {
  const codiQrSrc = useCodiQrSrc(createdResult?.codiQrImage, createdResult?.codiQrString);
  const activeMethod = createdResult?.method ?? method;

  const shareText = createdResult
    ? `Hola ${createdResult.charge.customerName}, te comparto tu cobro por ${formatMxnCents(createdResult.charge.amountCents)} (${createdResult.charge.concept}). ${createdResult.paymentLinkUrl ?? ''}`.trim()
    : `Hola ${fallbackCustomerName}, te comparto tu cobro por ${formatMxnCents(previewAmountCents)} (${concept}).`;

  const whatsappHref = customerPhone
    ? `https://wa.me/52${customerPhone}?text=${encodeURIComponent(shareText)}`
    : undefined;

  async function handleCopyLink(link: string) {
    try {
      await navigator.clipboard.writeText(link);
      onNotify('Enlace de pago copiado.');
    } catch {
      onNotify('Copia el enlace manualmente desde el recuadro.');
    }
  }

  return (
    <>
      {createdResult?.paymentLinkUrl && (
        <div className="db-link-box" aria-label="Enlace de pago generado">
          {createdResult.paymentLinkUrl}
        </div>
      )}

      {createdResult?.method === 'CODI_PUSH' && (
        <div className="db-qr-panel">
          <BoltIcon />
          <strong>La solicitud se envió al teléfono del cliente</strong>
          <span className="db-muted">
            {createdResult.charge.customerName} recibe el cobro CoDi en su teléfono.
          </span>
        </div>
      )}

      {activeMethod === 'CODI_QR' && (
        <div className="db-qr-panel">
          {codiQrSrc ? (
            <img className="db-qr-img" src={codiQrSrc} alt="Código QR CoDi" />
          ) : (
            <CodiQrGraphic />
          )}
          <strong>Escanea desde la app de tu banco</strong>
          <span className="db-muted">Liquidación directa a tu CLABE {clabeLabel}.</span>
        </div>
      )}

      <div className="db-row-actions">
        {createdResult?.paymentLinkUrl && (
          <button
            type="button"
            className="db-btn-secondary"
            onClick={() => void handleCopyLink(createdResult.paymentLinkUrl!)}
          >
            <CopyIcon />
            <span>Copiar enlace</span>
          </button>
        )}
        {whatsappHref && (
          <a className="db-btn-accent" href={whatsappHref} target="_blank" rel="noreferrer">
            <ShareIcon />
            <span>Enviar por WhatsApp</span>
          </a>
        )}
        {extraActions}
      </div>
    </>
  );
}
