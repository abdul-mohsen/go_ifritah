# Afrita Go - Copilot Instructions

## Build, Test, and Lint

All commands are available in `package.json` via npm. The project uses Go 1.21 with Gorilla Mux, Godotenv, and JavaScript tooling for assets and tests.

### Quick Commands
- **Full check** (build + lint + test): `npm run check`
- **Lint Go code**: `npm run lint:go` (golangci-lint with custom config in `.golangci.yml`)
- **Format code**: `npm run format` (Prettier for JS/CSS/HTML templates)
- **Check formatting**: `npm run format:check`
- **Run Go tests**: `npm run test` or `npm run test:verbose`
- **Build Tailwind CSS**: `npm run build:css` or `npm run watch:css` (watches for changes)

### Running a Single Test
```bash
go test -v ./handlers -run TestNameHere
```

### Configuration
- Linting: `.golangci.yml` (disabled rules: exhaustruct, wrapcheck, varnamelen, funlen, cyclop, gocognit, gocyclo, lll)
- Formatting: `.prettierrc` and `.prettierignore`
- Environment: Copy `.env.example` to `.env`, override with `.env.local` (gitignored)
  - Key vars: `PORT`, `APP_DOMAIN`, `BACKEND_URL`, `AFRITA_TOKEN_DIR`

## High-Level Architecture

**Afrita Go** is the frontend layer of the Afrita business management platform. It serves as a single-page application built with Go and HTMX that proxies API requests to a separate backend service.

### System Components
- **Frontend** (this repo, afrita-go): Go HTTP server with template-based UI, static assets
- **Backend** (separate repo, ifritah-go): REST API at `http://backend:8090` (or configured via `BACKEND_URL`)
- **Database**: Managed by backend (frontend is stateless except for auth tokens)

### Request Flow
1. Client requests page/resource from frontend (`:8000`)
2. Frontend renders HTML templates with HTMX
3. HTMX requests trigger handlers in `handlers/` that may call backend APIs
4. Frontend proxies `/api/*` requests to backend
5. Responses rendered as HTML or JSON depending on request type

### Authentication & RBAC
- **JWT-based**: Frontend obtains tokens from backend `/api/v2/login`, persists them in filesystem (`AFRITA_TOKEN_DIR`)
- **Session middleware**: `middleware/auth.go` extracts JWT from cookies, validates, injects user into context
- **Token refresh**: Automatic background token cleanup in `helpers/auth_tokens.go`
- **Roles**: Admin, Manager, User (defined in `models/types.go`)
- **Permission checks**: Enforced via middleware in `handlers/rbac.go`, wrapped in `main.go` with helper functions

### Key Modules
- **handlers/** (50+ files): HTTP handler functions for all features (invoices, products, clients, ZATCA e-invoicing, WhatsApp, etc.)
- **middleware/**: Auth validation and RBAC enforcement
- **models/**: Type definitions (user roles, permission structs, API request/response models)
- **config/**: Template loading, environment initialization, custom template functions
- **helpers/**: Utility functions (auth token lifecycle, validation, rendering, API calls, caching)
- **templates/**: Go HTML templates (components/, layouts/, partials/, page templates)
- **static/**: Tailwind CSS output, JavaScript, images
- **tests/**: Playwright E2E tests (separate from Go unit tests)
- **resources/**: Localization (L() function provides Arabic/English translations)

### Database Concepts (Backend-Driven)
- Frontend doesn't directly access database; all data flows through backend APIs
- Key entities: Users, Clients, Products, Stores, Branches, Invoices, Purchase Bills, Orders, Cash Vouchers, Credit Notes, Suppliers
- ZATCA integration: E-invoicing for Saudi Arabia compliance

## Key Conventions

### Handler Patterns
- **Naming**: `HandleXXX` for GET, `HandleXXXPost` for POST (e.g., `HandleInvoices`, `HandleInvoicesPost`)
- **Function signature**: `func HandleXXX(w http.ResponseWriter, r *http.Request)`
- **Template rendering**: `helpers.RenderStandalone(w, "template-name", data)` or `helpers.RenderLayout(w, "template-name", data, "layout-name")`
- **Error responses**: Use `helpers.RenderError(w, r, statusCode, "Arabic error message")`

### RBAC Middleware Usage (in main.go)
Three convenience wrappers provided for handler registration:
```go
protect := func(resource, action string, h http.HandlerFunc) http.HandlerFunc {
    return handlers.RequirePermission(resource, action)(h).ServeHTTP
}
adminOnly := func(h http.HandlerFunc) http.HandlerFunc {
    return handlers.RequireRole(models.RoleAdmin)(h).ServeHTTP
}
managerUp := func(h http.HandlerFunc) http.HandlerFunc {
    return handlers.RequireRole(models.RoleAdmin, models.RoleManager)(h).ServeHTTP
}
// Register: router.HandleFunc("/users", adminOnly(handlers.HandleUsers)).Methods("GET")
```

### Template Structure
- **Layouts** (`templates/layouts/`): Page wrapper (header, nav, footer)
- **Components** (`templates/components/`): Reusable UI blocks (forms, tables, modals)
- **Partials** (`templates/partials/`): Small template fragments
- **Pages** (`templates/*.html`): Top-level pages that extend layouts
- **Template functions**: Available in all templates via `config.go` FuncMap (e.g., `L()` for translations, `dict` for passing data to sub-templates)

### HTMX Integration
- Pages use HTMX for dynamic updates without full page reloads
- Handlers return HTML fragments for HTMX to swap into DOM
- HATEOAS principles: responses contain links to related resources

### Error Handling
- **Backend errors**: Check response status codes and JSON error bodies
- **Form validation**: Server-side validation in handlers, rendered back with error messages in Arabic
- **Logging**: Use `log.Printf()` with emoji prefixes for visibility (e.g., `log.Printf("🔑 Attempting login...")`)

### Testing
- **Unit tests**: Named `*_test.go`, use `go test`
- **Table-driven tests**: Common pattern for multiple scenarios (see `*_test.go` files)
- **E2E tests**: Playwright in `tests/` and `e2e/` directories, run via CI workflows
- **Test isolation**: Each test file can have a `testmain_test.go` with setup/teardown via `TestMain()`

### Naming Conventions
- **Files**: snake_case (e.g., `auth_tokens.go`, `rbac_test.go`)
- **Functions**: camelCase, exported functions start uppercase
- **Constants**: UPPERCASE (used in RBAC permissions)
- **Variables**: camelCase
- **Template names**: kebab-case (e.g., "add-invoice", "client-detail")

### Environment & Configuration
- Load via `config.Initialize()` using `github.com/joho/godotenv`
- `.env.example` shows all required and optional variables
- `.env.local` (gitignored) overrides `.env` for local development
- Commonly accessed as `config.AppDomain`, `config.BackendDomain`, `config.Port`

### Common Patterns
- **Backend API calls**: Construct URL from `config.BackendDomain`, use `http.Post()` or `http.Get()`, handle JSON responses
- **Form data**: Use `r.ParseForm()`, access via `r.FormValue("fieldname")`
- **JSON responses**: Use `json.Marshal()` to create payload, `json.NewDecoder()` to parse response
- **User context**: Retrieve from session via `getUserFromSession(r)` (in `handlers/rbac.go`)
- **Async operations**: Use goroutines with cleanup (e.g., `go helpers.PeriodicTokenCleanup()`)

### Localization
- Arabic is the primary language (strings often hardcoded in Arabic)
- Use `L()` template function for translated strings (see `resources/` for localization data)
- Error messages typically in Arabic for user-facing feedback

## Debugging & Development

- **Server startup**: `go run main.go` (loads config, templates, token cleanup goroutine)
- **Hot reload**: Use Air (configured in `.air.toml`) for auto-restart on file changes
- **Static asset changes**: Run `npm run watch:css` to rebuild Tailwind on CSS/template changes
- **Test failures**: Check logs for backend connection errors (verify `BACKEND_URL` is correct)
- **Token persistence**: Tokens stored in `AFRITA_TOKEN_DIR` (default: OS user config directory)

## CI/CD Workflows

Located in `.github/workflows/`:
- **go-test.yml**: Runs `go test ./...`
- **go-vet.yml**: Runs `go vet`
- **e2e.yml**: Playwright E2E tests
- **deploy.yml**: Deployment pipeline
- **lighthouse.yml**, **pa11y.yml**: Performance/accessibility audits
- **htmlvalidate.yml**: HTML validation
- **codeql.yml**: Security scanning
