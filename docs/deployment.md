# Despliegue en Cloudflare

El frontend se publica como assets del Worker `contractors-platform`. Las rutas `/api` y `/api/*` se envían a la API Go en Cloudflare Containers. La configuración vive en `apps/edge/wrangler.jsonc`.

## Pipelines de GitHub Actions

- `ci.yml`: valida cada push a ramas distintas de `main` y cada pull request. Ejecuta tipos, pruebas, compilación, construcción de la imagen Go y empaquetado de Wrangler sin publicar. También puede ser llamado por el workflow de despliegue.
- `deploy.yml`: al hacer push a `main`, ejecuta primero el mismo CI y después publica Worker, assets y contenedor. Admite ejecución manual únicamente desde `main`. Usa el environment `production` y serializa los despliegues.

No se despliega si las verificaciones fallan. Si faltan credenciales, el job de despliegue termina con un mensaje que indica cuáles configurar; CI funciona sin ellas.

## Credenciales pendientes

Configurar en GitHub → Settings → Secrets and variables → Actions, o en el environment `production`:

- `CLOUDFLARE_API_TOKEN`: token de la cuenta destino con permisos para editar Workers y los recursos de Containers/registro de imágenes.
- `CLOUDFLARE_ACCOUNT_ID`: identificador de la cuenta destino.

Usar una cuenta con Workers Paid y Containers habilitado. El runner Ubuntu de GitHub proporciona Docker para construir y subir la imagen. No guardar secretos en el repositorio.

## Verificación local

Con Node 22, Go y Docker disponibles:

```bash
npm ci
npm run check
docker build -t contractors-platform-backend:local apps/backend
npm run deploy:dry-run
```

`deploy:dry-run` valida el empaquetado del Worker sin publicarlo; no sustituye la prueba de ejecución del contenedor. Para probar Worker y contenedor juntos, ejecutar `npm run dev:edge` con Docker activo. Para publicar manualmente después de configurar las credenciales y compilar el frontend: `npm run deploy`.

El backend guarda los datos en SQLite (`DATABASE_PATH=/data/app.db`, definido en `apps/backend/Dockerfile`) sobre el disco del contenedor. Los registros desaparecen cuando el contenedor se suspende (15 minutos sin tráfico), reinicia o se redespliega: sirve para demos y pruebas e2e, no como almacenamiento persistente. Para persistir, reemplazar el repositorio por D1 u otra base externa.

> [!WARNING]
> El `Dockerfile` del backend define `AUTH_MODE=demo` para facilitar demos de la hackatón (permite cuenta demo, omite verificación obligatoria de correo y autoriza restablecer PIN sin código). **No usar `AUTH_MODE=demo` con datos reales**; en un despliegue productivo real cambiar a `AUTH_MODE=strict` y configurar correo `SMTP_HOST`.

## Referencias

- [Workers y GitHub Actions](https://developers.cloudflare.com/workers/ci-cd/external-cicd/github-actions/)
- [Cloudflare Containers](https://developers.cloudflare.com/containers/)
- [Configuración de Containers](https://developers.cloudflare.com/containers/configuration/wrangler/)
