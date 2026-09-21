# Graph Report - backend  (2026-09-11)

## Corpus Check
- 233 files · ~164,798 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 2477 nodes · 7071 edges · 129 communities (116 shown, 13 thin omitted)
- Extraction: 93% EXTRACTED · 7% INFERRED · 0% AMBIGUOUS · INFERRED: 507 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Community Hubs (Navigation)
- Catalog Synchronization
- Siatservice Components
- Certificate Management
- CUFD Management
- CUFD Management
- Maintenance Services
- Database Models
- Certificate Management
- Catalog Synchronization
- HTTP Delivery
- HTTP Delivery
- HTTP Delivery
- Documentospreparados Components
- Certificate Management
- Github Com Ron86I Go
- Fiscal Adapter
- Catalog Synchronization
- Facturacion Test
- Contingency Management
- Credential Encryption
- CUFD Management
- Invoice Processing
- Database Models
- Fiscal Package Processing
- Invoice Processing
- Catalog Synchronization
- Encoding Json Rawmessage
- Compras Test
- Fromsiatactividades Components
- Facturacion Components
- CUFD Management
- CUFD Management
- Invoice Processing
- Company Components
- Fiscal Package Processing
- Fusionarcamposlegados Components
- Catalog Synchronization
- Certificate Management
- Certificate Management
- Database Models
- Github Com Ron86I Go
- Catalog Synchronization
- Fromsiatresultadodocumento Components
- HTTP Delivery
- Database Models
- Catalog Synchronization
- Emission Queue
- Database Models
- Sectores Components
- CUFD Management
- Maintenance Services
- HTTP Delivery
- Invoice Processing
- Invoice Repository
- Invoice Repository
- Fiscal Package Processing
- Catalog Synchronization
- Certificate Management
- Invoice Processing
- Observability Stack
- Fiscal Integrity Migration
- Database Models
- Invoice Processing
- Github Com Ron86I Go
- Catalog Synchronization
- Catalog Synchronization
- Invoice Processing
- Campos Detalle Test
- Catalog Synchronization
- CUFD Management
- Invoice PDF Generation
- Maintenance Services
- Catalog Synchronization
- Invoice Processing
- Siat Batches Test
- Seed Main
- Codigoambiente Components
- Documento Ajuste
- Database Models
- Catalog Synchronization
- CUFD Management
- Certificate Management
- Cafc Y Sector Hotel
- Emission Queue
- Emission Queue
- CUIS Management
- Certificate Management
- Invoice Processing
- Catalog Synchronization
- Emission Queue
- Catalog Synchronization
- Invoice Processing
- Invoice Processing
- HTTP Delivery
- Badrequesterror Components
- Tosiatcompras Components
- Catalog Synchronization
- Catalog Synchronization
- Invoice Processing
- Observability Stack
- CUFD Management
- CUIS Management
- Firma Xmldsig Pendiente
- Test Main
- Fachadas Components
- CUIS Management
- Emisi N S Ncrona
- Product Mapping
- Investigaci N Web De
- Contrato P Blico Estable
- Observability Stack
- CUFD Management
- Points Of Sale
- Points Of Sale
- Points Of Sale
- Github Com Brandsrx Supay

## God Nodes (most connected - your core abstractions)
1. `Container` - 98 edges
2. `Company` - 74 edges
3. `Invoice` - 72 edges
4. `RespondError()` - 71 edges
5. `WriteJSON()` - 58 edges
6. `NewBadRequestError()` - 56 edges
7. `PointOfSale` - 56 edges
8. `SectorProfile` - 55 edges
9. `SiatUsecase` - 50 edges
10. `NewConflictError()` - 48 edges

## Surprising Connections (you probably didn't know these)
- `Datos sectoriales de cabecera y detalle` --semantically_similar_to--> `Campos sectoriales de detalle`  [INFERRED] [semantically similar]
  docs/invoicing-sectors.md → ARCHITECURE.md
- `Multi-tenancy inexistente en servicio SIAT actual` --semantically_similar_to--> `Multi-tenancy por diseño`  [INFERRED] [semantically similar]
  ARCHITECURE.md → PLAN.md
- `Catálogos SIAT sincronizados` --semantically_similar_to--> `catalog_versions`  [INFERRED] [semantically similar]
  DOCUMENTACION_SIAT.md → PLAN.md
- `Contingencia y emisión offline` --semantically_similar_to--> `contingency_events`  [INFERRED] [semantically similar]
  DOCUMENTACION_SIAT.md → PLAN.md
- `Auditoría y conservación fiscal` --semantically_similar_to--> `invoice_events`  [INFERRED] [semantically similar]
  DOCUMENTACION_SIAT.md → PLAN.md

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Flujo de emisión fiscal SIAT** — documentacion_siat_catalogos_sincronizados, documentacion_siat_cuis, documentacion_siat_cufd, documentacion_siat_cuf, documentacion_siat_firma_digital, documentacion_siat_auditoria_fiscal [EXTRACTED 1.00]
- **Jerarquía operativa multi-tenant** — plan_tenants, plan_branches, plan_points_of_sale, plan_cuis_history, plan_cufd_history [EXTRACTED 1.00]
- **Agregado fiscal de factura** — plan_invoices, plan_invoice_items, plan_invoice_events, plan_contingency_events, plan_cufd_history [EXTRACTED 1.00]

## Communities (129 total, 13 thin omitted)

### Community 0 - "Catalog Synchronization"
Cohesion: 0.05
Nodes (62): handler, catalogService, handler, Module, handler, handler, handler, net/http.Request (+54 more)

### Community 1 - "Siatservice Components"
Cohesion: 0.06
Nodes (50): Config, encoding/pem.Block, github.com/ron86i/go-siat/v2.Config, github.com/ron86i/go-siat/v2.CredentialSign, github.com/ron86i/go-siat/v2.SiatServices, extraerResultadoCompras(), Service, extraerResultadoEvento() (+42 more)

### Community 2 - "Certificate Management"
Cohesion: 0.09
Nodes (53): enforce_customer_immutability, api_keys, branches, catalog_sync_states, catalogs, certificates, companies, contingency_events (+45 more)

### Community 3 - "CUFD Management"
Cohesion: 0.13
Nodes (53): emittedInvoice(), newFakeInvoiceRepo(), newTestUsecase(), TestAnnulAccepted(), TestAnnulCaeAlCufdDeEmisionSinVigente(), TestAnnulMotivoInvalido(), TestAnnulNoAccepted(), TestAnnulRechazado() (+45 more)

### Community 4 - "CUFD Management"
Cohesion: 0.07
Nodes (48): encoding/xml.Name, testing.T, SanitizeCufd(), TestLaPazOffset(), TestSanitizeCufd(), TestSIATWallClockToInstant(), TestFacadeRoutingIgnoresHistoricalField(), TestRegistroValidaMetadatosDeBuilders() (+40 more)

### Community 5 - "Maintenance Services"
Cohesion: 0.06
Nodes (30): net/http.Client, net/http.Response, sync/atomic.Uint64, NewWebhookNotifier(), TestWebhookNotifierPostsNotification(), TestWebhookNotifierRejectsInvalidTarget(), Config, MaintenanceRepository (+22 more)

### Community 6 - "Database Models"
Cohesion: 0.09
Nodes (38): Branch, ContingencyEvent, gorm.io/datatypes.JSON, time.Time, DebeUsarVentanaHolgada(), FormatDisplayLaPaz(), VentanaContingenciaHolgada(), Cufd (+30 more)

### Community 7 - "Certificate Management"
Cohesion: 0.11
Nodes (8): Container, net/http.Handler, net/http.Server, NewContainer(), BranchRepository, Notifier, NewPostgresBranchRepository(), NewBranchUsecase()

### Community 8 - "Catalog Synchronization"
Cohesion: 0.10
Nodes (41): chi.Mux, TestSiatBatchExplicitEmptySelectionIsNotAutomatic(), TestSiatBatchResponseLimitsCompanyAndPointOfSaleData(), newHandler(), handler, newTestRouter(), responseObjectID(), TestCatalogReadiness() (+33 more)

### Community 9 - "HTTP Delivery"
Cohesion: 0.07
Nodes (37): net/http.HandlerFunc, sync.Mutex, time.Ticker, ApiKeyLookup, contextKey, RateLimiterIP, tokenBucket, v1Module (+29 more)

### Community 10 - "HTTP Delivery"
Cohesion: 0.07
Nodes (14): Module, chi.Router, handler, NewModule(), Branch, toDomainBranch(), isUniqueViolation(), toDomainPointOfSale() (+6 more)

### Community 11 - "HTTP Delivery"
Cohesion: 0.07
Nodes (20): handler, Module, fakeApiKeyLookup, newHandler(), chi.Router, handler, NewModule(), Company (+12 more)

### Community 12 - "Documentospreparados Components"
Cohesion: 0.11
Nodes (15): documentosPreparados(), FiscalAdapter, fromSiatResultadoPaquete(), toSiatMasiva(), toSiatPaquete(), TestFiscalServiceConcurrentCodesAreUnique(), TestFiscalServiceContract(), FiscalService (+7 more)

### Community 13 - "Certificate Management"
Cohesion: 0.08
Nodes (17): recordingMaintenanceRunner, FiscalService, context.Context, CompanyIDFromContext(), WithCompanyID(), fromSiatResultadoFirma(), Service, compress() (+9 more)

### Community 14 - "Github Com Ron86I Go"
Cohesion: 0.09
Nodes (28): github.com/ron86i/go-siat/v2/pkg/models.AnulacionFactura, github.com/ron86i/go-siat/v2/pkg/models.RecepcionDocumentoAjuste, github.com/ron86i/go-siat/v2/pkg/models.RecepcionFactura, github.com/ron86i/go-siat/v2/pkg/models.ReversionAnulacionFactura, github.com/ron86i/go-siat/v2/pkg/models.ValidacionRecepcionMasivaFactura, github.com/ron86i/go-siat/v2/pkg/models.ValidacionRecepcionPaqueteFactura, github.com/ron86i/go-siat/v2/pkg/models.VerificacionEstadoFactura, Service (+20 more)

### Community 15 - "Fiscal Adapter"
Cohesion: 0.10
Nodes (26): fromSiatFiscalItems(), fromSiatMensajes(), fromSiatResultadoDocumentoAjuste(), fromSiatResultadoEmision(), fromSiatResultadoEvento(), MarshalFiscalMessages(), toSiatDocumentoAjuste(), toSiatEvento() (+18 more)

### Community 16 - "Catalog Synchronization"
Cohesion: 0.10
Nodes (13): Product, ProductMapping, ProductMapping, NewPostgresProductRepository(), toDomainProduct(), TestBranchCustomerAndPointOfSaleLifecycles(), TestCompanyUsecaseLifecycleAndValidation(), TestProductUsecaseValidatesCatalogsAndPersistsMappings() (+5 more)

### Community 17 - "Facturacion Test"
Cohesion: 0.11
Nodes (33): assertFacturaXMLSinXsiNil(), buildSolicitudNota(), Service, newSignedTestService(), ptrStr(), TestBuildNotaCreditoDebitoDebitoPayload(), TestBuildNotaCreditoDebitoPayload(), TestEmitirFacturaCompraVentaPayload() (+25 more)

### Community 18 - "Contingency Management"
Cohesion: 0.10
Nodes (16): SiatLeyendaRepository, ContingencyEventRepository, CatalogSyncStateRepository, FiscalService, NewPostgresContingencyEventRepository(), documentTypeCodeToString(), CreateInvoiceRequest, InvoiceUsecase (+8 more)

### Community 19 - "Credential Encryption"
Cohesion: 0.11
Nodes (21): golang.org/x/sync/singleflight.Group, sync.RWMutex, SiatClientProvider, NewSiatClientProvider(), NewSiatClientProviderWithStorage(), newTestMemoryStorage(), TestProviderDecryptAndBuildService(), TestProviderLocalWithoutStorageFallback() (+13 more)

### Community 20 - "CUFD Management"
Cohesion: 0.12
Nodes (19): CatalogReadiness, SincronizacionResumen, ComprasInput, ComprasResultado, CufdResultado, CuisResultado, DocumentoAjusteResultado, EventoSignificativoInput (+11 more)

### Community 21 - "Invoice Processing"
Cohesion: 0.13
Nodes (18): InvoiceUsecase, InvoicePreview, InvoiceUsecase, MinimalInvoiceRequest, NewInvoiceRequestSimplifier(), normalizeAlias(), normalizeDocumentType(), normalizeInvoiceType() (+10 more)

### Community 22 - "Database Models"
Cohesion: 0.16
Nodes (30): github.com/gpdf-dev/gpdf/template.PageBuilder, InvoiceDocument, InvoiceEvent, EmissionType, Invoice, codigoPuntoVenta(), cufText(), currencyText() (+22 more)

### Community 23 - "Fiscal Package Processing"
Cohesion: 0.17
Nodes (15): InvoiceStatus, Company, SentPackage, SentPackageStatus, PointOfSale, recordBatchTransition(), sameOptionalInt64(), sameOptionalString() (+7 more)

### Community 24 - "Invoice Processing"
Cohesion: 0.14
Nodes (27): createTestUsecaseBuilder(), InvoiceUsecase, intPtr(), newFakeCustomerRepo(), strPtr(), TestCreatePurgeaCamposEducativosFueraDeSector11(), TestCreateRaceIdempotencia(), TestCreateWithReceiver() (+19 more)

### Community 25 - "Catalog Synchronization"
Cohesion: 0.08
Nodes (29): Multi-tenancy inexistente en servicio SIAT actual, Logs estructurados sin PII, Anulación fiscal trazable, Auditoría y conservación fiscal, Catálogos SIAT sincronizados, Contingencia y emisión offline, Firma digital fiscal, Paquete de facturas offline (+21 more)

### Community 26 - "Encoding Json Rawmessage"
Cohesion: 0.16
Nodes (25): encoding/json.RawMessage, reflect.Type, reflect.Value, aEntero(), aFlotante(), aplicarCamposDetalle(), construirCabecera(), construirDetalle() (+17 more)

### Community 27 - "Compras Test"
Cohesion: 0.12
Nodes (25): TestEnviarComprasCompletaIdentidadDesdeConfig(), TestEnviarComprasPayload(), TestEnviarComprasValidaLimite(), TestEnviarComprasValidaPeriodo(), TestEnviarComprasValidaPuntoVenta(), validSolicitudCompras(), TestRegistrarEventoSignificativo(), TestRegistrarEventoSignificativoRejected() (+17 more)

### Community 28 - "Fromsiatactividades Components"
Cohesion: 0.15
Nodes (19): fromSiatActividades(), fromSiatActividadesDocSector(), fromSiatLeyendas(), fromSiatParametricas(), fromSiatRespuestaSincronizacion(), fromSiatSinProducts(), toSiatSolicitudSincronizacion(), FiscalActivity (+11 more)

### Community 29 - "Facturacion Components"
Cohesion: 0.19
Nodes (17): buildFacturaSDK(), codigoDocumentoSectorXML(), empaquetaArchivo(), extraerResultadoFacturacion(), Service, parseNit(), removeEmptyOptionalFacturaFields(), resultadoDocumento() (+9 more)

### Community 30 - "CUFD Management"
Cohesion: 0.11
Nodes (11): Company, Cufd, Customer, Invoice, InvoiceItem, InvoiceListFilter, InvoiceDocument, InvoiceEvent (+3 more)

### Community 31 - "CUFD Management"
Cohesion: 0.12
Nodes (13): fromSiatRespuestaCufd(), fromSiatRespuestaCuis(), toSiatSolicitudCufd(), toSiatSolicitudCuis(), RespuestaCufd, RespuestaCuis, CredentialRequest, CufdResult (+5 more)

### Community 32 - "Invoice Processing"
Cohesion: 0.17
Nodes (10): NewConflictError(), clienteFromCustomer(), codigoTipoDocumentoIdentidad(), InvoiceUsecase, invoiceTransitionEvent(), isSIATConnectivityError(), marshalMensajes(), siatEstadoToDomain() (+2 more)

### Community 33 - "Company Components"
Cohesion: 0.13
Nodes (5): Company, fakeCompanyRepo, fakeCompanyRepo, phase12CompanyRepo, stubCompanyRepo

### Community 34 - "Fiscal Package Processing"
Cohesion: 0.18
Nodes (10): BatchInvoiceDocument, FiscalBatchRepository, SentPackageType, fiscalBatchRequest(), SiatUsecase, packageRequest(), prepareBatchPayloads(), PaqueteResultado (+2 more)

### Community 35 - "Fusionarcamposlegados Components"
Cohesion: 0.17
Nodes (18): fusionarCamposLegados(), normalizeCompraVenta(), nullSectorFields(), parseSectorObject(), presentSectorFields(), roundMoney(), TestRoundMoneyUsesTwoDecimals(), TestSectorDocumentDistingueAusenteDeNulo() (+10 more)

### Community 36 - "Catalog Synchronization"
Cohesion: 0.20
Nodes (4): ParseFiscalSyncOperation(), SiatUsecase, toCatalogoResultado(), CatalogoResultado

### Community 37 - "Certificate Management"
Cohesion: 0.11
Nodes (9): Certificate, Company, CertificateAlertTarget, CredentialTarget, PointOfSale, certificateWebhookURL(), notificationTestKey(), PostgresMaintenanceRepository (+1 more)

### Community 38 - "Certificate Management"
Cohesion: 0.15
Nodes (7): CertificateStatus, Company, Certificate, NewPostgresCertificateRepository(), toDomainCertificate(), PostgresCertificateRepository, fakeCertRepo

### Community 39 - "Database Models"
Cohesion: 0.18
Nodes (7): Customer, toDomainCustomer(), documentKey(), DocumentType, PostgresCustomerRepository, fakeCustomerRepo, phase12CustomerRepo

### Community 40 - "Github Com Ron86I Go"
Cohesion: 0.23
Nodes (10): github.com/ron86i/go-siat/v2/pkg/models.RecepcionPaqueteFactura, recuperarDocumentosLote(), validarIdentidadLote(), empaquetarXMLPersistidos(), extraerArchivoPaquete(), Service, validarXMLPersistido(), paquetePreparada (+2 more)

### Community 41 - "Catalog Synchronization"
Cohesion: 0.20
Nodes (18): expectedSOAPService(), TestAllSDKProfilesSendThroughTheirSOAPService(), TestPartialReturnSerializesOriginalAndReturnedAmounts(), assertSDKRoots(), countBuilderProfiles(), sdkCatalogCases(), TestFacadeSelectorsAreExclusive(), TestSectorRegistryParityWithSDKCatalog() (+10 more)

### Community 42 - "Fromsiatresultadodocumento Components"
Cohesion: 0.21
Nodes (7): fromSiatResultadoDocumento(), Service, FiscalAdapter, NewFiscalAdapter(), toSiatSolicitudDocumento(), FiscalDocumentQuery, FiscalDocumentResult

### Community 43 - "HTTP Delivery"
Cohesion: 0.16
Nodes (10): chi.Router, handler, NewModule(), NewNotFoundError(), PointOfSaleRepository, PointOfSaleUsecase, NewPointOfSaleUsecase(), Module (+2 more)

### Community 44 - "Database Models"
Cohesion: 0.17
Nodes (10): TipoPuntoVenta, CatalogItem, CatalogVersion, catalogMetadata(), latestCatalogItems(), replaceVersionedCatalog(), metadataString(), PostgresSiatActividadRepository (+2 more)

### Community 45 - "Catalog Synchronization"
Cohesion: 0.16
Nodes (13): gorm.io/gorm.DB, CatalogSyncState, NewPostgresCatalogSyncStateRepository(), crearCompanyYPos(), TestSiatCatalogosDedicadosRoundTrip(), TestSinProductReplaceDeduplica(), TestUpsertSyncStateIdempotente(), NewPostgresCustomerRepository() (+5 more)

### Community 46 - "Emission Queue"
Cohesion: 0.15
Nodes (11): github.com/prometheus/client_golang/prometheus.CounterVec, github.com/prometheus/client_golang/prometheus.Gauge, github.com/prometheus/client_golang/prometheus.HistogramVec, github.com/prometheus/client_golang/prometheus.Registry, HTTPMiddleware(), boundedResult(), DefaultMetrics(), Metrics (+3 more)

### Community 47 - "Database Models"
Cohesion: 0.19
Nodes (8): github.com/shopspring/decimal.Decimal, decimalFloat(), invoiceMutableFields(), positiveDecimalOrDefault(), toDomainInvoice(), toModelInvoice(), InvoiceStatus, PostgresInvoiceRepository

### Community 48 - "Sectores Components"
Cohesion: 0.17
Nodes (13): FacadeFija(), FacadePorModalidad(), facadeSelectorFor(), init(), normalizarValorCampo(), toFloat(), FacadeSelector, FachadaSDK (+5 more)

### Community 49 - "CUFD Management"
Cohesion: 0.16
Nodes (5): PointOfSale, fakeCredentialMaintainer, fakeCredentialProvider, fakeCredPOSStore, fakePointOfSaleRepo

### Community 50 - "Maintenance Services"
Cohesion: 0.20
Nodes (12): MaintenanceConfig, QueueConfig, R2Config, SiatInfraConfig, time.Duration, getEnv(), Config, Load() (+4 more)

### Community 51 - "HTTP Delivery"
Cohesion: 0.15
Nodes (10): Module, chi.Router, handler, NewModule(), CustomerRepository, CustomerUsecase, NewCustomerUsecase(), validDocumentType() (+2 more)

### Community 52 - "Invoice Processing"
Cohesion: 0.17
Nodes (10): EmissionQueue, InvoiceEmissionPayload, Dispatcher, fakePublisher, OutboxPublisher, OutboxRepository, NewDispatcher(), outboxRetryDelay() (+2 more)

### Community 53 - "Invoice Repository"
Cohesion: 0.17
Nodes (6): Cufd, NewPostgresCufdRepository(), toDomainCufd(), PostgresCufdRepository, fakeCredCufdStore, fakeCufdRepo

### Community 54 - "Invoice Repository"
Cohesion: 0.32
Nodes (15): NewPostgresInvoiceRepository(), newTestDB(), nuevaFacturaPendiente(), ptrString(), seedFixture(), TestClaimForEmissionEsAtomica(), TestClaimStatusTransicionCondicional(), TestCreateAsignaCorrelativosPorPuntoDeVenta() (+7 more)

### Community 55 - "Fiscal Package Processing"
Cohesion: 0.16
Nodes (13): SiatActividadRepository, CufdRepository, InvoiceRepository, SentPackageRepository, TipoPuntoVentaRepository, NewPostgresSentPackageRepository(), NewPostgresSiatActividadRepository(), NewPostgresTipoPuntoVentaRepository() (+5 more)

### Community 56 - "Catalog Synchronization"
Cohesion: 0.18
Nodes (6): httpCatalogRepo, TestCatalogRoutesDomainSlugs(), CatalogItem, NewPostgresCatalogRepository(), PostgresCatalogRepository, fakeCatalogRepo

### Community 57 - "Certificate Management"
Cohesion: 0.19
Nodes (11): Module, github.com/brandsrx/supay/internal/storage.CertStorage, chi.Router, handler, NewModule(), CertificateRepository, CompanyRepository, CertificateUsecase (+3 more)

### Community 58 - "Invoice Processing"
Cohesion: 0.17
Nodes (11): fakeRiverInserter, InvoiceEmissionArgs, InvoiceEmissionProcessor, InvoiceEmissionWorker, github.com/riverqueue/river.InsertOpts, github.com/riverqueue/river.Job, github.com/riverqueue/river.JobArgs, github.com/riverqueue/river/rivertype.JobInsertResult (+3 more)

### Community 59 - "Observability Stack"
Cohesion: 0.20
Nodes (12): log/slog.Attr, log/slog.Handler, log/slog.Level, log/slog.Record, attrsToAny(), hashValue(), isIdentifierKey(), isSensitiveKey() (+4 more)

### Community 60 - "Fiscal Integrity Migration"
Cohesion: 0.23
Nodes (16): prevent_hard_delete(), trg_no_delete_branches, trg_no_delete_certificates, trg_no_delete_contingency_events, trg_no_delete_cufd_history, trg_no_delete_cuis_history, trg_no_delete_customers, trg_no_delete_invoice_documents (+8 more)

### Community 61 - "Database Models"
Cohesion: 0.23
Nodes (9): InvoiceDocumentType, InvoiceDocument, InvoiceDocumentRepository, InvoiceDocument, NewPostgresInvoiceDocumentRepository(), rotateInvoiceDocument(), toDomainInvoiceDocuments(), persistInvoiceDocuments() (+1 more)

### Community 62 - "Invoice Processing"
Cohesion: 0.18
Nodes (9): CircuitBreaker, circuitState, fakeProcessor, NewCircuitBreaker(), NewInvoiceEmissionWorker(), TestExponentialRetryPolicyRespetaTope(), TestWorkerCancelaRechazoSIATSinReintentar(), TestWorkerReprogramaFacturaEnProceso() (+1 more)

### Community 63 - "Github Com Ron86I Go"
Cohesion: 0.29
Nodes (5): github.com/ron86i/go-siat/v2/pkg/models.RecepcionMasivaFactura, extraerArchivoMasiva(), Service, masivaPreparada, SolicitudMasivaFactura

### Community 64 - "Catalog Synchronization"
Cohesion: 0.24
Nodes (8): CatalogRepository, SiatActividadDocSectorRepository, ProductRepository, SinProductRepository, ProductUsecase, NewProductUsecase(), CreateProductRequest, ProductMappingRequest

### Community 65 - "Catalog Synchronization"
Cohesion: 0.22
Nodes (5): SinProduct, toDomainSinProduct(), PostgresSinProductRepository, ProductosSinResult, recordingSinProductRepo

### Community 66 - "Invoice Processing"
Cohesion: 0.20
Nodes (5): fakeOutboxRepo, OutboxEvent, NewPostgresOutboxRepository(), outboxToDomain(), PostgresOutboxRepository

### Community 67 - "Campos Detalle Test"
Cohesion: 0.19
Nodes (12): baseItemConstruyeFactura(), TestBuildFacturaRechazaDetalleRequeridoAusente(), TestItemSinDatosSectorNoCambiaXML(), TestValidarDatosDetalle(), TestBuildFacturaSDKRechazaModalidadNoHabilitada(), TestCompraVentaAdapterNormalizaPayloadTipado(), TestCompraVentaBuilderSeleccionaModalidad(), allSDKFields() (+4 more)

### Community 68 - "Catalog Synchronization"
Cohesion: 0.21
Nodes (4): SiatActividadDocSector, PostgresSiatActividadDocSectorRepository, fakeDocSectorRepo, recordingDocSectorRepo

### Community 69 - "CUFD Management"
Cohesion: 0.30
Nodes (5): CredentialService, NewCredentialServiceWithProvider(), resolveCodigoPuntoVenta(), CredentialCufdStore, CredentialPosStore

### Community 70 - "Invoice PDF Generation"
Cohesion: 0.26
Nodes (6): Service, NewService(), NewServiceWithStorage(), NewStorageFromConfig(), Storage, NewNoopStorage()

### Community 71 - "Maintenance Services"
Cohesion: 0.16
Nodes (8): maintenanceRunner, main(), SetupLogging(), runMaintenanceJob(), StartMaintenanceScheduler(), TestMaintenanceSchedulerRunsBothJobsOnStartup(), StartStaleEmissionReaper(), RunServer()

### Community 72 - "Catalog Synchronization"
Cohesion: 0.23
Nodes (5): SiatLeyenda, PostgresSiatLeyendaRepository, fakeLeyendaRepo, LeyendasFacturaResult, recordingLeyendaRepo

### Community 73 - "Invoice Processing"
Cohesion: 0.25
Nodes (11): buildDatosSectorNotaDescuento(), cloneItemsFromReferencia(), datosDetalleOriginal(), esSectorAjuste(), InvoiceUsecase, jsonFloat(), sameFiscalLine(), sumItemSubtotals() (+3 more)

### Community 74 - "Siat Batches Test"
Cohesion: 0.26
Nodes (13): batchFixture(), SiatUsecase, TestFalloPersistenciaResultadoConservaRecepcionEnRespuesta(), TestMasivaAgrupaYDivideSegunPerfilYLimite(), TestMasivaErrorEnUltimoLoteNoReservaNiEnvia(), TestMasivaPreparaYGuardaDocumentosAntesDeEnviar(), TestMasivaRechazaSeleccionNoAptaAntesDePreparar(), TestMasivaRespuestaInciertaConservaIdentidadYNoReenvia() (+5 more)

### Community 75 - "Seed Main"
Cohesion: 0.21
Nodes (9): getenv(), main(), gorm.io/gorm/logger.LogLevel, ConnectDB(), gormLogLevel(), Migrate(), MigrateDB(), NewPostgresCompanyRepository() (+1 more)

### Community 76 - "Codigoambiente Components"
Cohesion: 0.32
Nodes (6): SiatEnvironment, CompanyUsecase, validEnvironment(), validWebhookURL(), RegisterCompanyRequest, UpdateCompanyRequest

### Community 77 - "Documento Ajuste"
Cohesion: 0.35
Nodes (5): Service, ResultadoDocumentoAjuste, SolicitudAnulacionDocumentoAjuste, SolicitudDocumentoAjuste, TipoNota

### Community 78 - "Database Models"
Cohesion: 0.29
Nodes (4): ContingencyEvent, ContingencyReason, PostgresContingencyEventRepository, fakeContingencyRepo

### Community 79 - "Catalog Synchronization"
Cohesion: 0.35
Nodes (3): NewBadRequestError(), SiatUsecase, mustParametricItems()

### Community 80 - "CUFD Management"
Cohesion: 0.42
Nodes (11): NewCredentialService(), credFixtures(), TestEnsureCufdAusenteSolicitaYPersiste(), TestEnsureCufdEncadenaCuisCuandoFalta(), TestEnsureCufdExistenteNoLlamaSIAT(), TestEnsureCufdRechazoSiatEsConflicto(), TestEnsureCuisAusenteSolicitaYPersiste(), TestEnsureCuisProximoAVencerSeRenueva() (+3 more)

### Community 81 - "Certificate Management"
Cohesion: 0.29
Nodes (4): tenantSettingsWithCertificateWebhook(), toDomainCompany(), TestTenantSettingsWithCertificateWebhookPreservesOtherSettings(), PostgresCompanyRepository

### Community 82 - "Cafc Y Sector Hotel"
Cohesion: 0.20
Nodes (10): CAFC y sector Hotel como referencia, Campos sectoriales de detalle, Pipeline único de serialización fiscal, Registro sectorial, Supay — núcleo sectorial, Verificación exhaustiva contra el SDK, Validación cross-tenant de factura referenciada, Datos sectoriales de cabecera y detalle (+2 more)

### Community 83 - "Emission Queue"
Cohesion: 0.22
Nodes (6): context.CancelFunc, database/sql.DB, database/sql.Tx, github.com/riverqueue/river.Client, StartEmissionQueue(), Service

### Community 84 - "Emission Queue"
Cohesion: 0.29
Nodes (7): tenantBucket, TenantRateLimiter, NewTenantRateLimiter(), TestCircuitBreakerAislaTenantYAdmiteUnaSonda(), TestTenantRateLimiterAislaBuckets(), TestTenantRateLimiterConcurrenteNoComparteCuota(), timeNow()

### Community 85 - "CUIS Management"
Cohesion: 0.29
Nodes (5): Cuis, CuisRepository, NewPostgresCuisRepository(), toDomainCuis(), PostgresCuisRepository

### Community 86 - "Certificate Management"
Cohesion: 0.27
Nodes (4): NewCompanyUsecase(), TestCompanyRejectsInvalidCertificateWebhook(), TestCompanyUpdateConfiguresCertificateWebhook(), webhookCompanyRepo

### Community 87 - "Invoice Processing"
Cohesion: 0.31
Nodes (4): InvoiceEvent, InvoiceEventRepository, NewPostgresInvoiceEventRepository(), PostgresInvoiceEventRepository

### Community 88 - "Catalog Synchronization"
Cohesion: 0.33
Nodes (5): httpActividadRepo, SiatActividad, ActividadesEconomicasResult, CompanyActividadesResult, recordingActividadRepo

### Community 89 - "Emission Queue"
Cohesion: 0.33
Nodes (7): Config, ExponentialRetryPolicy, riverInserter, RiverPublisher, github.com/riverqueue/river/rivertype.JobRow, NewExponentialRetryPolicy(), NewService()

### Community 90 - "Catalog Synchronization"
Cohesion: 0.33
Nodes (8): ParseCodigoActividadInt64(), ResolveCatalogTipo(), TestResolveCatalogTipo(), CatalogCodigoDesc, CatalogItemsResult, DocumentoSectorItem, DocumentosSectorResult, EmisionBootstrapResult

### Community 91 - "Invoice Processing"
Cohesion: 0.32
Nodes (4): chi.Router, handler, NewModule(), Module

### Community 92 - "Invoice Processing"
Cohesion: 0.36
Nodes (6): BatchResultado, batchCompanyResponse, batchPointOfSaleResponse, batchResponse, batchSubmissionRequest, invoiceSelection

### Community 93 - "HTTP Delivery"
Cohesion: 0.33
Nodes (4): Module, chi.Router, handler, NewModule()

### Community 94 - "Badrequesterror Components"
Cohesion: 0.29
Nodes (3): BadRequestError, ConflictError, NotFoundError

### Community 95 - "Tosiatcompras Components"
Cohesion: 0.48
Nodes (3): toSiatCompras(), FiscalPurchase, FiscalPurchaseResult

### Community 96 - "Catalog Synchronization"
Cohesion: 0.48
Nodes (6): catalogs, siat_actividades, siat_actividades_doc_sector, siat_leyendas_factura, tenants, tipo_punto_ventas

### Community 97 - "Catalog Synchronization"
Cohesion: 0.43
Nodes (6): SiatUsecase, newPersistTestUsecase(), TestListCatalogTipos(), TestPersistSincronizacionRutasDedicadas(), TestResolveDocumentoSectorDesdeTabla(), toFiscalSyncResult()

### Community 98 - "Invoice Processing"
Cohesion: 0.60
Nodes (4): InvoiceStateMachine, InvoiceTransitionReason, transitionAllowed(), InvoiceStatus

### Community 99 - "Observability Stack"
Cohesion: 0.50
Nodes (4): Proveedor de dashboards Supay en Grafana, Datasource Prometheus de Grafana, Scrape Prometheus de Supay backend, Métricas Prometheus de Supay

### Community 100 - "CUFD Management"
Cohesion: 0.50
Nodes (4): CUF, CUFD, CUIS, Punto de venta SIAT

### Community 102 - "Firma Xmldsig Pendiente"
Cohesion: 0.50
Nodes (4): Firma XMLDSig pendiente, Motor de facturación SIAT inicial, Recepción de factura por SOAP, Generación y persistencia de XML fiscal

### Community 105 - "CUIS Management"
Cohesion: 0.67
Nodes (3): Aprovisionamiento SIAT de punto de venta, CUIS transitorio de sucursal, Punto de venta operativo

## Knowledge Gaps
- **40 isolated node(s):** `github.com/brandsrx/supay`, `Service`, `ctxKey`, `Service`, `SectorLayout` (+35 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **13 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Container` connect `Certificate Management` to `Siatservice Components`, `Maintenance Services`, `HTTP Delivery`, `HTTP Delivery`, `HTTP Delivery`, `Contingency Management`, `Credential Encryption`, `Catalog Synchronization`, `HTTP Delivery`, `Catalog Synchronization`, `Maintenance Services`, `HTTP Delivery`, `Invoice Processing`, `Fiscal Package Processing`, `Certificate Management`, `Database Models`, `Catalog Synchronization`, `CUFD Management`, `Invoice PDF Generation`, `Codigoambiente Components`, `Emission Queue`, `Invoice Processing`?**
  _High betweenness centrality (0.100) - this node is a cross-community bridge._
- **Why does `SectorProfile` connect `Github Com Ron86I Go` to `Fusionarcamposlegados Components`, `Campos Detalle Test`, `Github Com Ron86I Go`, `Catalog Synchronization`, `Invoice Processing`, `Sectores Components`, `Encoding Json Rawmessage`, `Facturacion Components`, `Github Com Ron86I Go`?**
  _High betweenness centrality (0.048) - this node is a cross-community bridge._
- **Why does `PointOfSale` connect `CUFD Management` to `Catalog Synchronization`, `Invoice Processing`, `Fiscal Package Processing`, `Catalog Synchronization`, `Maintenance Services`, `Database Models`, `CUFD Management`, `HTTP Delivery`, `HTTP Delivery`, `CUFD Management`, `Catalog Synchronization`, `CUFD Management`, `Encoding Json Rawmessage`?**
  _High betweenness centrality (0.039) - this node is a cross-community bridge._
- **What connects `github.com/brandsrx/supay`, `Service`, `ctxKey` to the rest of the system?**
  _40 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Catalog Synchronization` be split into smaller, more focused modules?**
  _Cohesion score 0.051299008030231456 - nodes in this community are weakly interconnected._
- **Should `Siatservice Components` be split into smaller, more focused modules?**
  _Cohesion score 0.05844155844155844 - nodes in this community are weakly interconnected._
- **Should `Certificate Management` be split into smaller, more focused modules?**
  _Cohesion score 0.08743169398907104 - nodes in this community are weakly interconnected._