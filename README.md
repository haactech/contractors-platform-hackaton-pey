# Plataforma de contratistas

Monorepo con npm workspaces y TypeScript estricto. Frontend en React + Vite y backend en Go (biblioteca estándar `net/http`). Cada aplicación organiza sus funcionalidades por dominio, dentro de módulos independientes.

## Desarrollo

Requiere Go 1.24 o superior, Node 22.12 o superior de la rama 22 y npm 10 o superior. `.nvmrc` selecciona Node 22.

```bash
nvm install
nvm use
npm ci
npm run dev
```

- Frontend: http://localhost:5173
- Backend: http://127.0.0.1:3001/api/health
- Contratistas: http://127.0.0.1:3001/api/contractors

Vite redirige `/api` al backend durante desarrollo. El inicio de sesión y registro crea cuentas con correo, teléfono y PIN de 6 dígitos (sesión mediante cookie `HttpOnly` `praxis_session`) ligadas 1:1 al registro de onboarding. Por defecto se guardan en memoria; con `DATABASE_PATH=/ruta/app.db` el backend usa SQLite (Docker y Cloudflare ya lo activan). El listado de contratistas es una colección vacía de arranque.

## Autenticación y modo demo

`AUTH_MODE` controla la política de acceso:

- **`strict` (default del binario Go):** exige verificar el correo (código de 6 dígitos), bloquea 15 min tras 5 PIN incorrectos y exige código por correo para recuperar el PIN.
- **`demo` (activo en `npm run dev`, Docker Compose y Dockerfile de Cloudflare):** omite la verificación obligatoria y el bloqueo, permite cambiar el PIN sin código, devuelve `devCode` para probar el flujo en la UI y siembra automáticamente la cuenta demo:
  - Correo: `demo@praxis.mx` · Teléfono: `+52 55 0000 0000` · PIN: `123456` (o botón **Probar con cuenta demo**).
  - La cuenta demo no permite cambiar su PIN y **no debe usarse con datos reales**.

Para probar el flujo estricto en desarrollo: `AUTH_MODE=strict npm run dev`. Variables detalladas en [.env.example](.env.example).

## Docker

Alternativa a instalar Go y Node: requiere Docker con Compose v2.

```bash
npm run docker:up      # o: docker compose up --build
npm run docker:down    # o: docker compose down
```

- Frontend (Vite con recarga en caliente): http://localhost:5173
- Backend: http://localhost:3001/api/health

El código se monta como volumen, así que los cambios del frontend se recargan solos. El backend corre con `go run`; tras cambios Go ejecuta `docker compose restart backend`. Si esos puertos están ocupados, cámbialos con `FRONTEND_PORT`, `BACKEND_PORT`, `API_PORT` o `WEB_PORT` (p. ej. `FRONTEND_PORT=5174 docker compose up`). Las dependencias de Node y los cachés de Go viven en volúmenes de Docker (no se mezclan con el `node_modules` del host). La base SQLite vive en el volumen `backend-data`; `docker compose down -v` la borra y deja datos limpios para pruebas e2e. Si cambias `package.json`, ejecuta `docker compose down -v` y vuelve a levantar. Dentro de Compose el proxy de Vite apunta al servicio `backend` mediante `VITE_API_PROXY_TARGET`; sin Docker usa `http://127.0.0.1:3001`.

Perfil prod-like, con el frontend compilado en nginx (SPA y proxy `/api`) y la imagen de producción de la API:

```bash
npm run docker:prod    # web en http://localhost:8081, API en http://localhost:8080
docker compose --profile prod down
```

## Estructura

```text
apps/
  frontend/src/
    app/                     # Composición y estilos de la aplicación
    modules/contractors/
      domain/                # Modelo local, independiente de React y HTTP
      application/           # Puertos de acceso a datos
      infrastructure/        # Adaptador HTTP
      presentation/          # Página y estados de la interfaz
      index.tsx              # API pública y composición del módulo
  backend/
    cmd/api/                 # Arranque y cierre del servidor
    internal/app/            # Composición HTTP
    internal/modules/onboarding/
      domain/                # Modelo, validación y puerto de repositorio
      application/           # Casos de uso
      infrastructure/        # Repositorios en memoria y SQLite
      presentation/          # Adaptador HTTP
      module.go              # API pública del módulo
packages/
  contracts/                 # Tipos de transporte compartidos, sin runtime
```

## Comandos

```bash
npm run dev          # Ambas aplicaciones; Ctrl+C detiene ambas
npm run typecheck    # TypeScript y go vet
npm test             # Pruebas Go con detector de condiciones de carrera
npm run build        # Compila ambas aplicaciones
npm run check        # Tipos, pruebas y build; también se ejecuta en CI
```

También puedes ejecutar `npm run dev -w @platform/frontend` o `npm run dev -w @platform/backend`.

Después de compilar, `npm run start -w @platform/backend` inicia la API. Admite `PORT` (3001 por defecto) y `HOST` (127.0.0.1 por defecto) como variables del proceso. Si cambias el puerto, ajusta el proxy en `apps/frontend/vite.config.ts`.

El frontend compilado queda en `apps/frontend/dist`. En producción, el servidor web debe servir esos archivos y redirigir `/api` al backend; el proxy de Vite solo funciona en desarrollo. `npm run preview -w @platform/frontend` únicamente permite inspeccionar los archivos compilados.

Consulta [las reglas de arquitectura](docs/architecture.md), el [flujo funcional de cobros y recordatorios](docs/workflow.md), el [diagrama interactivo de workflow](docs/FLUJO_WORKFLOW.html), el [prototipo funcional Desktop/Móvil](docs/prototipo/index.html) y la [referencia de integración con `api.pey.mx` (Payment Links y CoDi QR)](docs/pey-api.md) antes de agregar módulos.

## Contrato de autenticación y onboarding

Especificación: [OpenAPI 3.1](docs/openapi.json). Código y campos en inglés; interfaz y errores de validación en español. `client` representa al primer cliente del contratista.

- `POST /api/auth/signup`, `POST /api/auth/login`, `POST /api/auth/demo`, `POST /api/auth/logout`, `GET /api/auth/session`: manejo de cuentas y cookie de sesión `praxis_session`.
- `POST /api/onboarding`: requiere sesión (y correo verificado en `strict`), crea el registro ligado 1:1 a la cuenta, responde `201` y `Location` (`409` si la cuenta ya tiene registro).
- `GET /api/onboarding/{id}`: requiere sesión dueña del registro; responde `200`, `401`, `403` o `404`.
- Solo `taxRegime` es opcional. Se validan campos obligatorios, formato de email, teléfono y sintaxis del RFC. No se verifica el RFC ante el SAT.

```bash
curl -i -c cookies.txt http://127.0.0.1:3001/api/auth/signup \
  -H 'Content-Type: application/json' \
  -d '{"email":"ana@example.com","phone":"+52 55 1234 5678","pin":"123456"}'

curl -i -b cookies.txt http://127.0.0.1:3001/api/onboarding \
  -H 'Content-Type: application/json' \
  -d '{"name":"Ana Pérez","profession":"Electricista","client":{"name":"Luis López","phone":"+52 555 123 4567","email":"luis@example.com"},"businessName":"Servicios Pérez","businessActivity":"Instalaciones eléctricas","rfc":"AAA010101AAA","clabe":"002010077777777771"}'
```

Los errores usan `{ "message": "…", "errors": { "client.email": "…" } }`; `errors` aparece en validaciones (`422`). JSON inválido o campos desconocidos: `400`; contenido distinto de JSON: `415`; cuerpo mayor a 1 MiB: `413`. Los registros desaparecen al reiniciar el proceso. El onboarding todavía no crea una entrada en el listado de contratistas.

El backend se ejecuta con `go run` en desarrollo; después de cambios Go hay que reiniciarlo. Para trabajar sin npm: `cd apps/backend`, `go run ./cmd/api`, `go test -race ./...`.
