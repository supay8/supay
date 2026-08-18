# Graph Report - supay  (2026-08-14)

## Corpus Check
- cluster-only mode — file stats not available

## Summary
- 604 nodes · 1371 edges · 21 communities (19 shown, 2 thin omitted)
- Extraction: 96% EXTRACTED · 4% INFERRED · 0% AMBIGUOUS · INFERRED: 50 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `36fd58de`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- net/http.ResponseWriter
- testing.T
- context.Context
- Invoice
- time.Time
- devDependencies
- gorm.io/gorm.DB
- Mensaje
- PointOfSale
- compilerOptions
- CompanyRepository
- Company
- package.json
- Customer
- compilerOptions
- Load
- schema.sql
- tsconfig.json
- github.com/brandsrx/supay

## God Nodes (most connected - your core abstractions)
1. `newTestUsecase()` - 33 edges
2. `Invoice` - 30 edges
3. `newFakeInvoiceRepo()` - 29 edges
4. `Invoice` - 26 edges
5. `main()` - 25 edges
6. `testInvoice()` - 21 edges
7. `PointOfSale` - 19 edges
8. `SiatHandler` - 18 edges
9. `newTestService()` - 18 edges
10. `compilerOptions` - 18 edges

## Surprising Connections (you probably didn't know these)
- `newTestService()` --calls--> `NewService()`  [INFERRED]
  backend/internal/siat/service_test.go → backend/internal/siat/service.go
- `TestServiceSincronizarValidation()` --calls--> `SincronizacionOp`  [INFERRED]
  backend/internal/siat/sincronizacion_test.go → backend/internal/siat/sincronizacion.go
- `newTestUsecase()` --calls--> `NewInvoiceUsecase()`  [INFERRED]
  backend/internal/usecase/emission_test.go → backend/internal/usecase/invoice_usecase.go
- `TestCodigoTipoDocumentoIdentidad()` --calls--> `codigoTipoDocumentoIdentidad()`  [INFERRED]
  backend/internal/usecase/emission_test.go → backend/internal/usecase/emission.go
- `TestSiatEstadoToDomain()` --calls--> `siatEstadoToDomain()`  [INFERRED]
  backend/internal/usecase/emission_test.go → backend/internal/usecase/emission.go

## Import Cycles
- None detected.

## Communities (21 total, 2 thin omitted)

### Community 0 - "net/http.ResponseWriter"
Cohesion: 0.06
Nodes (30): main(), NewBranchHandler(), NewCompanyHandler(), NewCustomerHandler(), writeJSONError(), NewInvoiceHandler(), NewPosHandler(), NewRouter() (+22 more)

### Community 1 - "testing.T"
Cohesion: 0.12
Nodes (54): TestRegistrarEventoSignificativo(), TestRegistrarEventoSignificativoRejected(), TestRegistrarEventoSignificativoValidation(), formatFechaSiat(), TestEmitirFacturaCompraVentaPayload(), TestFormatFechaSiat(), newTestService(), TestServiceSolicitarCUFD() (+46 more)

### Community 2 - "context.Context"
Cohesion: 0.10
Nodes (32): empaquetaArchivo(), extraerResultadoFacturacion(), ResultadoDocumento, ResultadoEmision, Service, SolicitudDocumento, SolicitudFactura, parseNit() (+24 more)

### Community 3 - "Invoice"
Cohesion: 0.07
Nodes (28): CatalogRepository, Cufd, Cufd, Customer, Invoice, InvoiceRepository, InvoiceStatus, PointOfSale (+20 more)

### Community 4 - "time.Time"
Cohesion: 0.12
Nodes (40): Cufd, Customer, Branch, Company, Cufd, Customer, Invoice, PointOfSale (+32 more)

### Community 5 - "devDependencies"
Cohesion: 0.05
Nodes (38): plugins, rules, react/only-export-components, react/rules-of-hooks, $schema, dependencies, react, react-dom (+30 more)

### Community 6 - "gorm.io/gorm.DB"
Cohesion: 0.07
Nodes (18): CatalogItem, Cuis, CuisRepository, TipoPuntoVenta, TipoPuntoVentaRepository, Service, NewService(), NewPostgresCatalogRepository() (+10 more)

### Community 7 - "Mensaje"
Cohesion: 0.09
Nodes (22): extraerResultadoEvento(), ResultadoEventoSignificativo, Service, Mensaje, RespuestaCufd, RespuestaCuis, buildCredentialSign(), Service (+14 more)

### Community 8 - "PointOfSale"
Cohesion: 0.15
Nodes (11): PointOfSale, PointOfSaleRepository, isForeignKeyViolation(), isUniqueViolation(), NewPostgresPointOfSaleRepository(), toDomainPointOfSale(), PointOfSaleUsecase, NewPointOfSaleUsecase() (+3 more)

### Community 9 - "compilerOptions"
Cohesion: 0.08
Nodes (23): compilerOptions, allowArbitraryExtensions, allowImportingTsExtensions, erasableSyntaxOnly, jsx, lib, module, moduleDetection (+15 more)

### Community 10 - "CompanyRepository"
Cohesion: 0.16
Nodes (9): Branch, BranchRepository, CompanyRepository, toDomainBranch(), BranchUsecase, NewBranchUsecase(), PostgresBranchRepository, CreateBranchRequest (+1 more)

### Community 11 - "Company"
Cohesion: 0.17
Nodes (10): Company, SiatEnvironment, toDomainCompany(), CompanyUsecase, NewCompanyUsecase(), validEnvironment(), SiatEnvironment, PostgresCompanyRepository (+2 more)

### Community 12 - "package.json"
Cohesion: 0.10
Nodes (20): author, description, devDependencies, turbo, devEngines, packageManager, keywords, license (+12 more)

### Community 13 - "Customer"
Cohesion: 0.19
Nodes (10): Customer, CustomerRepository, NewPostgresCustomerRepository(), toDomainCustomer(), CustomerUsecase, NewCustomerUsecase(), validDocumentType(), DocumentType (+2 more)

### Community 14 - "compilerOptions"
Cohesion: 0.10
Nodes (19): compilerOptions, allowImportingTsExtensions, erasableSyntaxOnly, lib, module, moduleDetection, noEmit, noFallthroughCasesInSwitch (+11 more)

### Community 15 - "Load"
Cohesion: 0.14
Nodes (12): getenv(), main(), getEnv(), Load(), parseDuration(), parseInt64(), ConnectDB(), NewPostgresCompanyRepository() (+4 more)

### Community 16 - "schema.sql"
Cohesion: 0.46
Nodes (7): branches, catalogs, cufds, cuis, point_of_sales, tipo_punto_ventas, companies

## Knowledge Gaps
- **74 isolated node(s):** `siatEventoSignificativoRequest`, `author`, `description`, `keywords`, `license` (+69 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **2 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `main()` connect `net/http.ResponseWriter` to `Invoice`, `gorm.io/gorm.DB`, `Mensaje`, `PointOfSale`, `CompanyRepository`, `Company`, `Customer`, `Load`?**
  _High betweenness centrality (0.104) - this node is a cross-community bridge._
- **Why does `Invoice` connect `Invoice` to `testing.T`, `time.Time`?**
  _High betweenness centrality (0.094) - this node is a cross-community bridge._
- **Why does `Invoice` connect `time.Time` to `Invoice`?**
  _High betweenness centrality (0.047) - this node is a cross-community bridge._
- **What connects `siatEventoSignificativoRequest`, `author`, `description` to the rest of the system?**
  _74 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `net/http.ResponseWriter` be split into smaller, more focused modules?**
  _Cohesion score 0.06368011847463902 - nodes in this community are weakly interconnected._
- **Should `testing.T` be split into smaller, more focused modules?**
  _Cohesion score 0.11584699453551912 - nodes in this community are weakly interconnected._
- **Should `context.Context` be split into smaller, more focused modules?**
  _Cohesion score 0.09585037989479836 - nodes in this community are weakly interconnected._