# Graph Report - backend  (2026-09-11)

## Corpus Check
- cluster-only mode — file stats not available

## Summary
- 2511 nodes · 7079 edges · 140 communities (120 shown, 20 thin omitted)
- Extraction: 93% EXTRACTED · 7% INFERRED · 0% AMBIGUOUS · INFERRED: 507 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `b9e7e902`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- withDynamicConfig
- emission_test.go
- MaintenanceService
- time.Time
- SiatUsecase
- seedFixture
- testing.T
- newHandler
- context.Context
- 000001_baseline.up.sql
- SectorProfile
- provider
- Certificate
- Container
- FiscalPackageResult
- gorm.io/gorm.DB
- newTestService
- facturacion_test.go
- fiscal_adapter.go
- FiscalAdapter
- .Create
- net/http.ResponseWriter
- Company
- SentPackage
- middleware.go
- Branch
- Invoice
- PerfilesSector
- ItemFactura
- SolicitudFactura
- invoices
- InvoiceEmissionWorker
- buildFacturaSDK
- Invoice
- toDomainInvoice
- Product
- config/config.go
- CufdResult
- Customer
- NewConflictError
- ProductUsecase
- TestCatalogRoutesDomainSlugs
- .Simplify
- fakeMaintenanceRepo
- encoding/json.RawMessage
- PerfilSectorLayout
- PointOfSale
- minimalTestInvoice
- batchFixture
- NewCircuitBreaker
- TenantMiddleware
- latestCatalogItems
- InvoiceUsecase
- CompanyRepository
- tenants
- tenants
- Metrics
- FiscalDocument
- builder_reflex.go
- Cufd
- Dispatcher
- InvoiceStatus
- NewNotFoundError
- Module
- CredentialService
- 000010_fiscal_data_integrity.up.sql
- PerfilSector
- CatalogItem
- .prepararMasiva
- SinProduct
- SiatActividadDocSector
- OutboxEvent
- SiatLeyenda
- ContingencyEvent
- Service
- SiatActividadDocSectorRepository
- Service
- time.Duration
- NewService
- NewBadRequestError
- toDomainPointOfSale
- NewCredentialService
- InvoiceDocument
- toDomainCompany
- Campos sectoriales de detalle
- Cuis
- NewCompanyUsecase
- .prepareBatch
- InvoiceEvent
- CatalogSyncStateRepository
- Module
- Module
- Module
- Module
- domain/errors.go
- .Insert
- NewRouter
- NewModule
- LocalStorage
- 000002_normalize_structure.down.sql
- .prepareInvoiceBatches
- CircuitBreaker
- 000006_phase8_maintenance.up.sql
- NewFiscalService
- versionedTestModule
- Datasource Prometheus de Grafana
- CUFD
- 000006_phase8_maintenance.down.sql
- Motor de facturación SIAT inicial
- test/main.go
- envioPorModalidad
- outbox
- CUIS transitorio de sucursal
- Emisión síncrona con fallback offline
- 000009_fiscal_batch_reservations.up.sql
- product_mappings
- Investigación web de integración SIAT
- Contrato público estable Supay API v1
- Errores HTTP accionables
- cufd_history
- points_of_sale
- points_of_sale
- points_of_sale
- github.com/brandsrx/supay
- main
- siat/context.go
- fakePDFGenerator

## God Nodes (most connected - your core abstractions)
1. `Container` - 97 edges
2. `Company` - 74 edges
3. `Invoice` - 72 edges
4. `RespondError()` - 71 edges
5. `WriteJSON()` - 58 edges
6. `PointOfSale` - 56 edges
7. `NewBadRequestError()` - 56 edges
8. `SectorProfile` - 55 edges
9. `SiatUsecase` - 50 edges
10. `NewConflictError()` - 48 edges

## Surprising Connections (you probably didn't know these)
- `Multi-tenancy inexistente en servicio SIAT actual` --semantically_similar_to--> `Multi-tenancy por diseño`  [INFERRED] [semantically similar]
  ARCHITECURE.md → PLAN.md
- `Catálogos SIAT sincronizados` --semantically_similar_to--> `catalog_versions`  [INFERRED] [semantically similar]
  DOCUMENTACION_SIAT.md → PLAN.md
- `Contingencia y emisión offline` --semantically_similar_to--> `contingency_events`  [INFERRED] [semantically similar]
  DOCUMENTACION_SIAT.md → PLAN.md
- `Auditoría y conservación fiscal` --semantically_similar_to--> `invoice_events`  [INFERRED] [semantically similar]
  DOCUMENTACION_SIAT.md → PLAN.md
- `Datos sectoriales de cabecera y detalle` --semantically_similar_to--> `Campos sectoriales de detalle`  [INFERRED] [semantically similar]
  docs/invoicing-sectors.md → ARCHITECURE.md

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Flujo de emisión fiscal SIAT** — documentacion_siat_catalogos_sincronizados, documentacion_siat_cuis, documentacion_siat_cufd, documentacion_siat_cuf, documentacion_siat_firma_digital, documentacion_siat_auditoria_fiscal [EXTRACTED 1.00]
- **Agregado fiscal de factura** — plan_invoices, plan_invoice_items, plan_invoice_events, plan_contingency_events, plan_cufd_history [EXTRACTED 1.00]
- **Jerarquía operativa multi-tenant** — plan_tenants, plan_branches, plan_points_of_sale, plan_cuis_history, plan_cufd_history [EXTRACTED 1.00]

## Communities (140 total, 20 thin omitted)

### Community 0 - "withDynamicConfig"
Cohesion: 0.05
Nodes (53): Config, encoding/pem.Block, github.com/ron86i/go-siat/v2.Config, github.com/ron86i/go-siat/v2.CredentialSign, github.com/ron86i/go-siat/v2.SiatServices, extraerResultadoCompras(), Service, extraerResultadoEvento() (+45 more)

### Community 1 - "emission_test.go"
Cohesion: 0.11
Nodes (62): createTestUsecaseBuilder(), emittedInvoice(), InvoiceUsecase, newFakeCustomerRepo(), newFakeInvoiceRepo(), newTestUsecase(), strPtr(), TestAnnulAccepted() (+54 more)

### Community 2 - "MaintenanceService"
Cohesion: 0.06
Nodes (30): net/http.Client, net/http.Response, sync/atomic.Uint64, NewWebhookNotifier(), TestWebhookNotifierPostsNotification(), TestWebhookNotifierRejectsInvalidTarget(), Config, MaintenanceRepository (+22 more)

### Community 3 - "time.Time"
Cohesion: 0.08
Nodes (39): Branch, ContingencyEvent, gorm.io/datatypes.JSON, time.Time, DebeUsarVentanaHolgada(), FormatDisplayLaPaz(), VentanaContingenciaHolgada(), Cufd (+31 more)

### Community 4 - "SiatUsecase"
Cohesion: 0.11
Nodes (19): toSiatSolicitudSincronizacion(), CatalogReadiness, FiscalSyncOperation, FiscalSyncRequest, FiscalSyncResult, ParseFiscalSyncOperation(), SincronizacionResumen, SetupResultado (+11 more)

### Community 5 - "seedFixture"
Cohesion: 0.08
Nodes (36): main(), gorm.io/gorm/logger.LogLevel, log/slog.Attr, log/slog.Handler, log/slog.Level, log/slog.Record, SetupLogging(), StartStaleEmissionReaper() (+28 more)

### Community 6 - "testing.T"
Cohesion: 0.08
Nodes (40): encoding/xml.Name, testing.T, SanitizeCufd(), TestLaPazOffset(), TestSanitizeCufd(), TestSIATWallClockToInstant(), TestFacadeRoutingIgnoresHistoricalField(), TestRegistroValidaMetadatosDeBuilders() (+32 more)

### Community 7 - "newHandler"
Cohesion: 0.10
Nodes (41): chi.Mux, TestSiatBatchExplicitEmptySelectionIsNotAutomatic(), TestSiatBatchResponseLimitsCompanyAndPointOfSaleData(), newHandler(), handler, newTestRouter(), responseObjectID(), TestCatalogReadiness() (+33 more)

### Community 8 - "context.Context"
Cohesion: 0.10
Nodes (17): recordingMaintenanceRunner, context.Context, FiscalBatchRepository, SiatUsecase, ComprasInput, CuisResultado, DocumentoAjusteInput, DocumentoAjusteResultado (+9 more)

### Community 9 - "000001_baseline.up.sql"
Cohesion: 0.09
Nodes (40): enforce_customer_immutability, api_keys, branches, catalog_sync_states, catalogs, certificates, companies, contingency_events (+32 more)

### Community 10 - "SectorProfile"
Cohesion: 0.09
Nodes (28): github.com/ron86i/go-siat/v2/pkg/models.AnulacionFactura, github.com/ron86i/go-siat/v2/pkg/models.RecepcionDocumentoAjuste, github.com/ron86i/go-siat/v2/pkg/models.RecepcionFactura, github.com/ron86i/go-siat/v2/pkg/models.ReversionAnulacionFactura, github.com/ron86i/go-siat/v2/pkg/models.ValidacionRecepcionMasivaFactura, github.com/ron86i/go-siat/v2/pkg/models.ValidacionRecepcionPaqueteFactura, github.com/ron86i/go-siat/v2/pkg/models.VerificacionEstadoFactura, Service (+20 more)

### Community 11 - "provider"
Cohesion: 0.12
Nodes (18): github.com/brandsrx/supay/internal/storage.CertStorage, golang.org/x/sync/singleflight.Group, sync.RWMutex, Service, SiatClientProvider, NewSiatClientProvider(), NewSiatClientProviderWithStorage(), Service (+10 more)

### Community 12 - "Certificate"
Cohesion: 0.09
Nodes (17): CertificateStatus, newTestMemoryStorage(), TestProviderDecryptAndBuildService(), TestProviderLocalWithoutStorageFallback(), TestProviderMissingTokenError(), TestProviderRejectsEncryptedP12WithWrongKey(), MustNew(), New() (+9 more)

### Community 13 - "Container"
Cohesion: 0.14
Nodes (9): Container, net/http.Handler, net/http.Server, NewContainer(), SentPackageRepository, TipoPuntoVentaRepository, NewPostgresPointOfSaleRepository(), NewPostgresSentPackageRepository() (+1 more)

### Community 14 - "FiscalPackageResult"
Cohesion: 0.13
Nodes (11): documentosPreparados(), FiscalAdapter, fromSiatResultadoPaquete(), toSiatMasiva(), toSiatPaquete(), toSiatSolicitudFacturas(), FiscalService, FiscalBulk (+3 more)

### Community 15 - "gorm.io/gorm.DB"
Cohesion: 0.09
Nodes (16): gorm.io/gorm.DB, InvoiceDocumentRepository, Company, ApiKey, PostgresApiKeyRepository, HashKey(), NewPostgresApiKeyRepository(), NewPostgresInvoiceDocumentRepository() (+8 more)

### Community 16 - "newTestService"
Cohesion: 0.12
Nodes (25): TestEnviarComprasCompletaIdentidadDesdeConfig(), TestEnviarComprasPayload(), TestEnviarComprasValidaLimite(), TestEnviarComprasValidaPeriodo(), TestEnviarComprasValidaPuntoVenta(), validSolicitudCompras(), TestRegistrarEventoSignificativo(), TestRegistrarEventoSignificativoRejected() (+17 more)

### Community 17 - "facturacion_test.go"
Cohesion: 0.11
Nodes (33): assertFacturaXMLSinXsiNil(), buildSolicitudNota(), Service, newSignedTestService(), ptrStr(), TestBuildNotaCreditoDebitoDebitoPayload(), TestBuildNotaCreditoDebitoPayload(), TestEmitirFacturaCompraVentaPayload() (+25 more)

### Community 18 - "fiscal_adapter.go"
Cohesion: 0.11
Nodes (26): fromSiatActividades(), fromSiatActividadesDocSector(), fromSiatLeyendas(), fromSiatMensajes(), fromSiatParametricas(), fromSiatRespuestaCuis(), fromSiatRespuestaSincronizacion(), fromSiatResultadoCompras() (+18 more)

### Community 19 - "FiscalAdapter"
Cohesion: 0.15
Nodes (11): fromSiatResultadoDocumento(), fromSiatResultadoFirma(), Service, FiscalAdapter, NewFiscalAdapter(), toSiatSolicitudDocumento(), FiscalDocumentQuery, FiscalDocumentResult (+3 more)

### Community 20 - ".Create"
Cohesion: 0.11
Nodes (20): InvoiceItem, buildDatosSectorNotaDescuento(), cloneItemsFromReferencia(), datosDetalleOriginal(), esSectorAjuste(), InvoiceUsecase, jsonFloat(), sameFiscalLine() (+12 more)

### Community 21 - "net/http.ResponseWriter"
Cohesion: 0.05
Nodes (63): handler, handler, catalogService, handler, handler, handler, net/http.Request, net/http.ResponseWriter (+55 more)

### Community 22 - "Company"
Cohesion: 0.11
Nodes (5): Company, fakeCompanyRepo, fakeCompanyRepo, phase12CompanyRepo, stubCompanyRepo

### Community 23 - "SentPackage"
Cohesion: 0.14
Nodes (21): BatchInvoiceDocument, Company, SentPackage, SentPackageStatus, SentPackageType, PointOfSale, recordBatchTransition(), sameOptionalInt64() (+13 more)

### Community 24 - "middleware.go"
Cohesion: 0.18
Nodes (15): sync.Mutex, time.Ticker, ApiKeyLookup, RateLimiterIP, tokenBucket, clientIP(), extractKeyPrefix(), newTokenBucket() (+7 more)

### Community 25 - "Branch"
Cohesion: 0.11
Nodes (11): Module, chi.Router, handler, NewModule(), Branch, toDomainBranch(), BranchUsecase, PostgresBranchRepository (+3 more)

### Community 26 - "Invoice"
Cohesion: 0.16
Nodes (30): github.com/gpdf-dev/gpdf/template.PageBuilder, InvoiceDocument, InvoiceEvent, EmissionType, Invoice, codigoPuntoVenta(), cufText(), currencyText() (+22 more)

### Community 27 - "PerfilesSector"
Cohesion: 0.21
Nodes (17): expectedSOAPService(), TestAllSDKProfilesSendThroughTheirSOAPService(), TestPartialReturnSerializesOriginalAndReturnedAmounts(), assertSDKRoots(), countBuilderProfiles(), sdkCatalogCases(), TestFacadeSelectorsAreExclusive(), TestSectorRegistryParityWithSDKCatalog() (+9 more)

### Community 28 - "ItemFactura"
Cohesion: 0.13
Nodes (19): Service, ItemFactura, fromSiatFiscalCustomer(), fromSiatFiscalItems(), fromSiatResultadoDocumentoAjuste(), toSiatDocumentoAjuste(), toSiatFiscalCustomer(), toSiatFiscalItems() (+11 more)

### Community 29 - "SolicitudFactura"
Cohesion: 0.19
Nodes (18): fusionarCamposLegados(), SolicitudFactura, normalizeCompraVenta(), nullSectorFields(), parseSectorObject(), presentSectorFields(), roundMoney(), TestSectorDocumentDistingueAusenteDeNulo() (+10 more)

### Community 30 - "invoices"
Cohesion: 0.08
Nodes (29): Multi-tenancy inexistente en servicio SIAT actual, Logs estructurados sin PII, Anulación fiscal trazable, Auditoría y conservación fiscal, Catálogos SIAT sincronizados, Contingencia y emisión offline, Firma digital fiscal, Paquete de facturas offline (+21 more)

### Community 31 - "InvoiceEmissionWorker"
Cohesion: 0.33
Nodes (7): InvoiceEmissionArgs, InvoiceEmissionProcessor, InvoiceEmissionWorker, github.com/riverqueue/river.Job, github.com/riverqueue/river.WorkerDefaults, isPermanentEmissionError(), NewInvoiceEmissionWorker()

### Community 32 - "buildFacturaSDK"
Cohesion: 0.19
Nodes (16): buildFacturaSDK(), codigoDocumentoSectorXML(), empaquetaArchivo(), extraerResultadoFacturacion(), Service, parseNit(), removeEmptyOptionalFacturaFields(), resultadoDocumento() (+8 more)

### Community 33 - "Invoice"
Cohesion: 0.10
Nodes (13): sampleInvoice(), strPtr(), Company, Cufd, Customer, Invoice, InvoiceListFilter, InvoiceDocument (+5 more)

### Community 34 - "toDomainInvoice"
Cohesion: 0.13
Nodes (10): github.com/shopspring/decimal.Decimal, decimalFloat(), invoiceMutableFields(), positiveDecimalOrDefault(), toDomainCufd(), toDomainInvoice(), toModelInvoice(), InvoiceStatus (+2 more)

### Community 35 - "Product"
Cohesion: 0.13
Nodes (9): Product, ProductMapping, ProductMapping, toDomainProduct(), TestProductUsecaseValidatesCatalogsAndPersistsMappings(), mappingSectors(), PostgresProductRepository, fakeProductRepository (+1 more)

### Community 36 - "config/config.go"
Cohesion: 0.22
Nodes (10): QueueConfig, R2Config, SiatInfraConfig, getEnv(), Config, Load(), parseBoolEnv(), parseDeploymentMode() (+2 more)

### Community 37 - "CufdResult"
Cohesion: 0.18
Nodes (7): fromSiatRespuestaCufd(), toSiatSolicitudCufd(), toSiatSolicitudCuis(), CredentialRequest, CufdResult, convertMensajes(), SolicitudCuis

### Community 38 - "Customer"
Cohesion: 0.16
Nodes (7): Customer, toDomainCustomer(), documentKey(), DocumentType, PostgresCustomerRepository, fakeCustomerRepo, phase12CustomerRepo

### Community 39 - "NewConflictError"
Cohesion: 0.16
Nodes (10): NewConflictError(), clienteFromCustomer(), codigoTipoDocumentoIdentidad(), InvoiceUsecase, invoiceTransitionEvent(), isSIATConnectivityError(), siatEstadoToDomain(), valueOrEmpty() (+2 more)

### Community 40 - "ProductUsecase"
Cohesion: 0.19
Nodes (10): CatalogRepository, ProductRepository, SinProductRepository, NewPostgresCatalogRepository(), NewPostgresProductRepository(), NewPostgresSinProductRepository(), ProductUsecase, NewProductUsecase() (+2 more)

### Community 41 - "TestCatalogRoutesDomainSlugs"
Cohesion: 0.25
Nodes (5): Module, TestCatalogRoutesDomainSlugs(), chi.Router, handler, NewModule()

### Community 42 - ".Simplify"
Cohesion: 0.20
Nodes (17): InvoiceUsecase, InvoicePreview, InvoiceUsecase, MinimalInvoiceRequest, NewInvoiceRequestSimplifier(), normalizeAlias(), normalizeDocumentType(), normalizeInvoiceType() (+9 more)

### Community 43 - "fakeMaintenanceRepo"
Cohesion: 0.11
Nodes (9): Certificate, Company, CertificateAlertTarget, CredentialTarget, PointOfSale, certificateWebhookURL(), notificationTestKey(), PostgresMaintenanceRepository (+1 more)

### Community 44 - "encoding/json.RawMessage"
Cohesion: 0.12
Nodes (18): encoding/json.RawMessage, allSDKFields(), TestEverySDKProfileBuildsWithAllExposedFields(), TestSectorDataRejectsFractionalIntegerAndWrongJSONObject(), FacadeFija(), FacadePorModalidad(), facadeSelectorFor(), init() (+10 more)

### Community 45 - "PerfilSectorLayout"
Cohesion: 0.19
Nodes (11): github.com/ron86i/go-siat/v2/pkg/models.RecepcionPaqueteFactura, recuperarDocumentosLote(), validarIdentidadLote(), empaquetarXMLPersistidos(), extraerArchivoPaquete(), Service, validarXMLPersistido(), PerfilSectorLayout() (+3 more)

### Community 46 - "PointOfSale"
Cohesion: 0.13
Nodes (8): PointOfSale, TestCompanyUsecaseLifecycleAndValidation(), ComprasResultado, CufdResultado, EventoSignificativoResultado, fakeCredPOSStore, fakePointOfSaleRepo, phase12POSRepo

### Community 47 - "minimalTestInvoice"
Cohesion: 0.16
Nodes (19): intPtr(), TestAutofillDocumentoAjusteDescuentoCopiaItemsYDatosSector(), TestAutofillDocumentoAjusteDescuentoRespetaCamposProveidos(), TestBuildDatosSectorNotaDescuentoCompletaCampos(), requiredSectorData(), seedOriginalInvoice(), TestV1AdjustmentCopiesCompatibleDataAndKeepsDuplicateLines(), TestV1AdjustmentRejectsAlteredOriginalMetadata() (+11 more)

### Community 48 - "batchFixture"
Cohesion: 0.26
Nodes (13): batchFixture(), SiatUsecase, TestFalloPersistenciaResultadoConservaRecepcionEnRespuesta(), TestMasivaAgrupaYDivideSegunPerfilYLimite(), TestMasivaErrorEnUltimoLoteNoReservaNiEnvia(), TestMasivaPreparaYGuardaDocumentosAntesDeEnviar(), TestMasivaRechazaSeleccionNoAptaAntesDePreparar(), TestMasivaRespuestaInciertaConservaIdentidadYNoReenvia() (+5 more)

### Community 49 - "NewCircuitBreaker"
Cohesion: 0.16
Nodes (13): fakeProcessor, tenantBucket, TenantRateLimiter, NewCircuitBreaker(), NewTenantRateLimiter(), TestCircuitBreakerAislaTenantYAdmiteUnaSonda(), TestTenantRateLimiterAislaBuckets(), TestTenantRateLimiterConcurrenteNoComparteCuota() (+5 more)

### Community 50 - "TenantMiddleware"
Cohesion: 0.23
Nodes (8): contextKey, fakeApiKeyLookup, WithCompanyID(), TenantMiddleware(), errRecordNotFound(), TestTenantMiddlewareAcceptsValidApiKey(), TestTenantMiddlewareRejectsInvalidApiKey(), TestTenantMiddlewareRejectsMissingHeader()

### Community 51 - "latestCatalogItems"
Cohesion: 0.16
Nodes (11): TipoPuntoVenta, CatalogItem, CatalogVersion, catalogMetadata(), latestCatalogItems(), replaceVersionedCatalog(), metadataInt(), metadataString() (+3 more)

### Community 52 - "InvoiceUsecase"
Cohesion: 0.18
Nodes (16): ContingencyEventRepository, CufdRepository, InvoiceRepository, PointOfSaleRepository, FiscalService, NewPostgresContingencyEventRepository(), NewPostgresCufdRepository(), TestBuildSincronizacionResumen() (+8 more)

### Community 53 - "CompanyRepository"
Cohesion: 0.10
Nodes (19): handler, Module, newHandler(), chi.Router, handler, NewModule(), BranchRepository, CompanyRepository (+11 more)

### Community 54 - "tenants"
Cohesion: 0.21
Nodes (20): branches, catalog_sync_states, certificate_notifications, certificates, contingency_events, cufd_history, cuis_history, customers (+12 more)

### Community 55 - "tenants"
Cohesion: 0.22
Nodes (20): branches, catalog_sync_states, certificate_notifications, certificates, contingency_events, cufd_history, cuis_history, customers (+12 more)

### Community 56 - "Metrics"
Cohesion: 0.15
Nodes (11): github.com/prometheus/client_golang/prometheus.CounterVec, github.com/prometheus/client_golang/prometheus.Gauge, github.com/prometheus/client_golang/prometheus.HistogramVec, github.com/prometheus/client_golang/prometheus.Registry, HTTPMiddleware(), boundedResult(), DefaultMetrics(), Metrics (+3 more)

### Community 57 - "FiscalDocument"
Cohesion: 0.40
Nodes (4): fromSiatResultadoEmision(), toSiatSolicitudFactura(), FiscalDocument, FiscalResult

### Community 58 - "builder_reflex.go"
Cohesion: 0.25
Nodes (18): reflect.Type, reflect.Value, aEntero(), aFlotante(), aplicarCamposDetalle(), construirCabecera(), construirDetalle(), construirDetalleConCodigo() (+10 more)

### Community 59 - "Cufd"
Cohesion: 0.15
Nodes (6): Cufd, invoiceDTO, fakeCredCufdStore, fakeCredentialMaintainer, fakeCredentialProvider, fakeCufdRepo

### Community 60 - "Dispatcher"
Cohesion: 0.17
Nodes (10): EmissionQueue, InvoiceEmissionPayload, Dispatcher, fakePublisher, OutboxPublisher, OutboxRepository, NewDispatcher(), outboxRetryDelay() (+2 more)

### Community 61 - "InvoiceStatus"
Cohesion: 0.20
Nodes (6): InvoiceStateMachine, InvoiceStatus, InvoiceTransitionReason, transitionAllowed(), InvoiceStatus, batchTestRepository

### Community 62 - "NewNotFoundError"
Cohesion: 0.17
Nodes (10): SiatEnvironment, NewNotFoundError(), CompanyUsecase, validEnvironment(), validWebhookURL(), PointOfSaleUsecase, RegisterCompanyRequest, RegisterPointOfSaleRequest (+2 more)

### Community 63 - "Module"
Cohesion: 0.48
Nodes (5): v1Module, Module, chi.Router, registerModules(), registerTenantRoutes()

### Community 64 - "CredentialService"
Cohesion: 0.25
Nodes (6): CuisResult, CuisNeedsRenewal(), CredentialService, NewCredentialServiceWithProvider(), CredentialCufdStore, CredentialPosStore

### Community 65 - "000010_fiscal_data_integrity.up.sql"
Cohesion: 0.23
Nodes (16): prevent_hard_delete(), trg_no_delete_branches, trg_no_delete_certificates, trg_no_delete_contingency_events, trg_no_delete_cufd_history, trg_no_delete_cuis_history, trg_no_delete_customers, trg_no_delete_invoice_documents (+8 more)

### Community 66 - "PerfilSector"
Cohesion: 0.21
Nodes (10): baseItemConstruyeFactura(), TestBuildFacturaRechazaDetalleRequeridoAusente(), TestItemSinDatosSectorNoCambiaXML(), TestValidarDatosDetalle(), TestBuildFacturaSDKRechazaModalidadNoHabilitada(), TestCompraVentaAdapterNormalizaPayloadTipado(), TestCompraVentaBuilderSeleccionaModalidad(), TestRoundMoneyUsesTwoDecimals() (+2 more)

### Community 67 - "CatalogItem"
Cohesion: 0.23
Nodes (4): httpCatalogRepo, CatalogItem, PostgresCatalogRepository, fakeCatalogRepo

### Community 68 - ".prepararMasiva"
Cohesion: 0.29
Nodes (5): github.com/ron86i/go-siat/v2/pkg/models.RecepcionMasivaFactura, extraerArchivoMasiva(), Service, masivaPreparada, SolicitudMasivaFactura

### Community 69 - "SinProduct"
Cohesion: 0.22
Nodes (5): SinProduct, toDomainSinProduct(), PostgresSinProductRepository, ProductosSinResult, recordingSinProductRepo

### Community 70 - "SiatActividadDocSector"
Cohesion: 0.21
Nodes (4): SiatActividadDocSector, PostgresSiatActividadDocSectorRepository, fakeDocSectorRepo, recordingDocSectorRepo

### Community 71 - "OutboxEvent"
Cohesion: 0.22
Nodes (5): fakeOutboxRepo, OutboxEvent, NewPostgresOutboxRepository(), outboxToDomain(), PostgresOutboxRepository

### Community 72 - "SiatLeyenda"
Cohesion: 0.15
Nodes (11): SiatLeyenda, SiatUsecase, newPersistTestUsecase(), TestListCatalogTipos(), TestPersistSincronizacionRutasDedicadas(), TestResolveDocumentoSectorDesdeTabla(), toFiscalSyncResult(), PostgresSiatLeyendaRepository (+3 more)

### Community 73 - "ContingencyEvent"
Cohesion: 0.23
Nodes (4): ContingencyEvent, ContingencyReason, PostgresContingencyEventRepository, fakeContingencyRepo

### Community 74 - "Service"
Cohesion: 0.27
Nodes (6): Service, NewService(), NewServiceWithStorage(), NewStorageFromConfig(), Storage, NewNoopStorage()

### Community 75 - "SiatActividadDocSectorRepository"
Cohesion: 0.21
Nodes (10): SiatActividadDocSectorRepository, SiatActividadRepository, SiatLeyendaRepository, crearCompanyYPos(), TestSiatCatalogosDedicadosRoundTrip(), TestSinProductReplaceDeduplica(), TestUpsertSyncStateIdempotente(), NewPostgresSiatActividadDocSectorRepository() (+2 more)

### Community 76 - "Service"
Cohesion: 0.22
Nodes (6): context.CancelFunc, database/sql.DB, database/sql.Tx, github.com/riverqueue/river.Client, StartEmissionQueue(), Service

### Community 77 - "time.Duration"
Cohesion: 0.27
Nodes (6): maintenanceRunner, MaintenanceConfig, time.Duration, runMaintenanceJob(), StartMaintenanceScheduler(), TestMaintenanceSchedulerRunsBothJobsOnStartup()

### Community 78 - "NewService"
Cohesion: 0.29
Nodes (7): Config, ExponentialRetryPolicy, riverInserter, RiverPublisher, github.com/riverqueue/river/rivertype.JobRow, NewExponentialRetryPolicy(), NewService()

### Community 79 - "NewBadRequestError"
Cohesion: 0.13
Nodes (17): httpActividadRepo, SiatActividad, NewBadRequestError(), SiatUsecase, mustParametricItems(), nombreDocumentoSector(), ParseCodigoActividadInt64(), ResolveCatalogTipo() (+9 more)

### Community 80 - "toDomainPointOfSale"
Cohesion: 0.26
Nodes (3): isUniqueViolation(), toDomainPointOfSale(), PostgresPointOfSaleRepository

### Community 81 - "NewCredentialService"
Cohesion: 0.42
Nodes (11): NewCredentialService(), credFixtures(), TestEnsureCufdAusenteSolicitaYPersiste(), TestEnsureCufdEncadenaCuisCuandoFalta(), TestEnsureCufdExistenteNoLlamaSIAT(), TestEnsureCufdRechazoSiatEsConflicto(), TestEnsureCuisAusenteSolicitaYPersiste(), TestEnsureCuisProximoAVencerSeRenueva() (+3 more)

### Community 82 - "InvoiceDocument"
Cohesion: 0.36
Nodes (6): InvoiceDocumentType, InvoiceDocument, rotateInvoiceDocument(), toDomainInvoiceDocuments(), persistInvoiceDocuments(), PostgresInvoiceDocumentRepository

### Community 83 - "toDomainCompany"
Cohesion: 0.29
Nodes (4): tenantSettingsWithCertificateWebhook(), toDomainCompany(), TestTenantSettingsWithCertificateWebhookPreservesOtherSettings(), PostgresCompanyRepository

### Community 84 - "Campos sectoriales de detalle"
Cohesion: 0.20
Nodes (10): CAFC y sector Hotel como referencia, Campos sectoriales de detalle, Pipeline único de serialización fiscal, Registro sectorial, Supay — núcleo sectorial, Verificación exhaustiva contra el SDK, Validación cross-tenant de factura referenciada, Datos sectoriales de cabecera y detalle (+2 more)

### Community 85 - "Cuis"
Cohesion: 0.29
Nodes (5): Cuis, CuisRepository, NewPostgresCuisRepository(), toDomainCuis(), PostgresCuisRepository

### Community 86 - "NewCompanyUsecase"
Cohesion: 0.27
Nodes (4): NewCompanyUsecase(), TestCompanyRejectsInvalidCertificateWebhook(), TestCompanyUpdateConfiguresCertificateWebhook(), webhookCompanyRepo

### Community 87 - ".prepareBatch"
Cohesion: 0.31
Nodes (4): FiscalService, compress(), FiscalService, preparedBatch

### Community 88 - "InvoiceEvent"
Cohesion: 0.31
Nodes (4): InvoiceEvent, InvoiceEventRepository, NewPostgresInvoiceEventRepository(), PostgresInvoiceEventRepository

### Community 89 - "CatalogSyncStateRepository"
Cohesion: 0.31
Nodes (4): CatalogSyncState, CatalogSyncStateRepository, NewPostgresCatalogSyncStateRepository(), PostgresCatalogSyncStateRepository

### Community 90 - "Module"
Cohesion: 0.32
Nodes (4): chi.Router, handler, NewModule(), Module

### Community 91 - "Module"
Cohesion: 0.33
Nodes (4): Module, chi.Router, handler, NewModule()

### Community 92 - "Module"
Cohesion: 0.33
Nodes (4): Module, chi.Router, handler, NewModule()

### Community 93 - "Module"
Cohesion: 0.33
Nodes (4): Module, chi.Router, handler, NewModule()

### Community 94 - "domain/errors.go"
Cohesion: 0.29
Nodes (3): BadRequestError, ConflictError, NotFoundError

### Community 95 - ".Insert"
Cohesion: 0.33
Nodes (5): fakeRiverInserter, github.com/riverqueue/river.InsertOpts, github.com/riverqueue/river.JobArgs, github.com/riverqueue/river/rivertype.JobInsertResult, TestRiverPublisherInsertaJobUnicoEnColaDeEmision()

### Community 96 - "NewRouter"
Cohesion: 0.25
Nodes (8): net/http.HandlerFunc, InternalBootstrapMiddleware(), LimitBody(), TestInternalBootstrapMiddleware(), TestLimitBodyRejectsOversizedPayload(), NewRouter(), TestRouterExponeHealthYBootstrapVersionados(), TestRouterExponeModulosBajoV1()

### Community 97 - "NewModule"
Cohesion: 0.33
Nodes (4): chi.Router, handler, NewModule(), Module

### Community 99 - "000002_normalize_structure.down.sql"
Cohesion: 0.48
Nodes (6): catalogs, siat_actividades, siat_actividades_doc_sector, siat_leyendas_factura, tenants, tipo_punto_ventas

### Community 100 - ".prepareInvoiceBatches"
Cohesion: 0.48
Nodes (5): fiscalBatchRequest(), packageRequest(), prepareBatchPayloads(), resolveCodigoPuntoVenta(), invoiceBatch

### Community 102 - "000006_phase8_maintenance.up.sql"
Cohesion: 0.33
Nodes (4): certificate_notifications, record_point_of_sale_cuis, tenants, trg_point_of_sale_cuis_history

### Community 103 - "NewFiscalService"
Cohesion: 0.50
Nodes (3): TestFiscalServiceConcurrentCodesAreUnique(), TestFiscalServiceContract(), NewFiscalService()

### Community 105 - "Datasource Prometheus de Grafana"
Cohesion: 0.50
Nodes (4): Proveedor de dashboards Supay en Grafana, Datasource Prometheus de Grafana, Scrape Prometheus de Supay backend, Métricas Prometheus de Supay

### Community 106 - "CUFD"
Cohesion: 0.50
Nodes (4): CUF, CUFD, CUIS, Punto de venta SIAT

### Community 108 - "Motor de facturación SIAT inicial"
Cohesion: 0.50
Nodes (4): Firma XMLDSig pendiente, Motor de facturación SIAT inicial, Recepción de factura por SOAP, Generación y persistencia de XML fiscal

### Community 112 - "CUIS transitorio de sucursal"
Cohesion: 0.67
Nodes (3): Aprovisionamiento SIAT de punto de venta, CUIS transitorio de sucursal, Punto de venta operativo

### Community 138 - "siat/context.go"
Cohesion: 0.50
Nodes (3): CompanyIDFromContext(), WithCompanyID(), ctxKey

## Knowledge Gaps
- **43 isolated node(s):** `Service`, `Service`, `sent_package_invoices`, `github.com/brandsrx/supay`, `FiscalBatchPreparer` (+38 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **20 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Container` connect `Container` to `withDynamicConfig`, `MaintenanceService`, `SiatUsecase`, `provider`, `gorm.io/gorm.DB`, `Branch`, `config/config.go`, `ProductUsecase`, `InvoiceUsecase`, `CompanyRepository`, `Dispatcher`, `NewNotFoundError`, `Module`, `CredentialService`, `Service`, `SiatActividadDocSectorRepository`, `Service`, `InvoiceEvent`, `CatalogSyncStateRepository`?**
  _High betweenness centrality (0.077) - this node is a cross-community bridge._
- **Why does `Invoice` connect `Invoice` to `emission_test.go`, `toDomainInvoice`, `time.Time`, `.prepareInvoiceBatches`, `seedFixture`, `NewConflictError`, `context.Context`, `.Simplify`, `encoding/json.RawMessage`, `NewBadRequestError`, `batchFixture`, `NewCircuitBreaker`, `.Create`, `net/http.ResponseWriter`, `Cufd`, `InvoiceStatus`?**
  _High betweenness centrality (0.054) - this node is a cross-community bridge._
- **Why does `NewBadRequestError()` connect `NewBadRequestError` to `Invoice`, `.prepareInvoiceBatches`, `SiatUsecase`, `NewConflictError`, `ProductUsecase`, `context.Context`, `.Simplify`, `provider`, `gorm.io/gorm.DB`, `toDomainPointOfSale`, `.Create`, `net/http.ResponseWriter`, `CompanyRepository`, `NewNotFoundError`, `Branch`, `Cufd`, `domain/errors.go`?**
  _High betweenness centrality (0.040) - this node is a cross-community bridge._
- **What connects `Service`, `Service`, `sent_package_invoices` to the rest of the system?**
  _43 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `withDynamicConfig` be split into smaller, more focused modules?**
  _Cohesion score 0.052100840336134456 - nodes in this community are weakly interconnected._
- **Should `emission_test.go` be split into smaller, more focused modules?**
  _Cohesion score 0.1141352063213345 - nodes in this community are weakly interconnected._
- **Should `MaintenanceService` be split into smaller, more focused modules?**
  _Cohesion score 0.06289308176100629 - nodes in this community are weakly interconnected._