Plan: arquitectura sectorial verificable (Supay V2)
Inspeccionado
Supay — núcleo sectorial
- internal/siat/sectores.go (registro, SectorProfile, SectorAdapter, validación de datos_sector)
- internal/siat/sectores_catalogo.go (catálogo de 51 perfiles: 50 con builder + 33 sin builder)
- internal/siat/builder_reflex.go (aplicador reflexivo de builders del SDK)
- internal/siat/sector_adapters.go (genericSectorAdapter, compraVentaAdapter)
- internal/siat/facturacion.go (EmitirFactura, buildFacturaSDK, removeEmptyOptionalFacturaFields)
- internal/siat/fachadas.go (enrutamiento SOAP por fachada)
- internal/siat/paquetes.go, internal/siat/masiva.go, internal/siat/service.go
- internal/usecase/emission.go, internal/usecase/invoice_usecase.go (creación/validación), cmd/server/main.go
- Tests: sector_sdk_parity_test.go, sectores_test.go, sector_registry*_test.go, facturacion_test.go, paquetes_test.go
go-siat v2.1.1
- siat.go (12 fachadas), docs/es/explanation/sectores.md (catálogo normativo de 51 códigos)
- pkg/models/facturacion.go:423,647 (WithFacturas), pkg/models/invoices/*.go (builders por sector)
- internal/core/domain/datatype/nilable.go, datatype/time.go
- Referencia real: facturaElectronicaCompraVenta.xml (ejemplo oficial SIAT, raíz del repo)
Hallazgos
1. El registro sectorial es la abstracción correcta y hay que conservarlo.
SectorProfile + catalogoSectores + SectorAdapter + fachadas.go ya cumplen el objetivo del §2: no hay if/switch sectoriales dispersos, el flujo es único (buildFacturaSDK → adapter → builder del SDK → firma → fachada) y agregar un sector es agregar una entrada. No hay que rediseñar esto. Los problemas están en la completitud y en la verificación, no en la forma.
2. Los 50 sectores están registrados pero estructuralmente incompletos. Este es el hallazgo central.
construirCabecera (builder_reflex.go:190-222) aplica 27 campos comunes y luego solo los declarados en p.Campos. Todo método With* del builder del SDK que no esté en ninguna de las dos listas nunca se llama, y el nodo viaja xsi:nil o vacío. Comparando la superficie real de los builders del SDK contra lo declarado en sectores_catalogo.go:
Sector	Campos de cabecera del SDK que Supay nunca setea
19 Hidrocarburos IEHD	montoIehd, ciudad, nombrePropietario, nombreRepresentanteLegal, condicionPago, periodoEntrega
14 Alcanzada ICE	montoIceEspecifico, montoIcePorcentual
18 Juegos de Azar	montoTotalIj, montoTotalSujetoIpj
34 Seguros	ajusteAfectacionIva
21 Venta de Minerales	~14 campos (concentradoGranel, origen, puertoDestino, incoterm, tipoCambioANB, kilosNetosSecos, gastosRealizacion, iva, …)
Declaran Campos vacío ~25 de los 50 perfiles. La emisión de esos sectores compila, pasa los tests actuales y produce un documento que el SIAT rechazará.
3. No existe ningún mecanismo para campos sectoriales de detalle.
ItemFactura (facturacion.go:32-42) tiene 9 campos fijos y construirDetalle (builder_reflex.go:291-317) aplica solo esos 9 en modo tolerante. Los builders de detalle del SDK exponen campos propios que hoy son inalcanzables desde la API de Supay: porcentajeIehd (19), codigoTipoHabitacion + detalleHuespedes (16), codigoNandina + descripcionLeyes + cantidadExtraccion + unidadMedidaExtraccion (21). Sin esto, los 50 sectores son inalcanzables por diseño, no por falta de entradas en el catálogo.
4. La omisión silenciosa es el mecanismo de fallo, y es doble.
construirCabecera:224 salta el campo si !tieneMetodo(cab, metodo); construirDetalle:311 llama con tolerante=true. Ambos casos son legítimos (montoTotal no existe en notas, municipio no existe en boletos) pero indistinguibles de un error real: un método renombrado en el SDK, un campo obligatorio no declarado o un typo producen exactamente el mismo resultado — un XML incompleto, sin error, sin log.
5. El único test de paridad verifica el nombre del elemento raíz, nada más.
sector_sdk_parity_test.go:148 compara root != mode.root. Los 50 sectores "pasan" comprobando que la raíz XML se llama como el SDK dice. Ningún test verifica que los campos del XSD del sector estén poblados. De ahí la sensación de cobertura total con cobertura real de ~4 sectores (1, 8, 11, 24/47/48).
6. Existen dos dialectos XML: la emisión individual y el paquete/masiva producen bytes distintos para la misma factura.
Individual (facturacion.go:186-202): xml.Marshal → removeEmptyOptionalFacturaFields → SignXML → CompressAndHash.
Paquete/masiva (paquetes.go:150, masiva.go:111): delegan a WithFacturas del SDK, que hace xml.Marshal → SignXML sin la limpieza. Es decir: los nodos xsi:nil que la ruta homologada elimina, la ruta de paquetes los envía. Esto es candidato directo a explicar "paquetes: parcial / masiva: parcial".
7. HEAD está en rojo y la ruta homologada individual está rota.
El commit 583c8c4 quitó "cafc" de la lista de removeEmptyOptionalFacturaFields (facturacion.go:340) y eliminó {"WithCafc", nil} de los campos comunes, para habilitar CAFC en facturas de contingencia. Resultado: hoy la emisión individual envía <cafc xsi:nil="true"> y fallan TestEmitirFacturaCompraVentaPayload y TestEmitirFacturaSectorEducativoPayload — precisamente los dos casos homologados. Además CAFC quedó sin forma de setearse: no existe campo Cafc en SolicitudFactura. El objetivo del commit no se logró y se rompió lo que funcionaba.
8. La limpieza de xsi:nil es un workaround por regex y su justificación está sin confirmar.
removeEmptyOptionalFacturaFields borra nodos con expresiones regulares sobre el XML ya serializado. El comentario dice que "el SIAT rechaza xsi:nil", pero el ejemplo oficial del SIAT (facturaElectronicaCompraVenta.xml, raíz del repo) contiene <cafc xsi:nil="true"/>, <codigoPuntoVenta xsi:nil="true"/>, <complemento xsi:nil="true"/> — o sea, el XSD los declara nillable y el comportamiento del SDK es correcto. Lo más probable es que se haya arrastrado al documento un problema que era del envelope del request (donde el prefijo xsi no está declarado; ver el workaround explícito WithCafc(&emptyStr) en paquetes.go:147). El stripping está empíricamente aceptado por el SIAT, así que se conserva — pero como decisión consciente y en un solo lugar, no como regex disperso.
9. Trampa latente en fechas sectoriales.
WithFechaIngresoHospedaje recibe time.Time y el SDK lo serializa con TimeSiat: si el valor es cero emite <fechaIngresoHospedaje></fechaIngresoHospedaje> (datatype/time.go:20), inválido para xs:dateTime. Supay declara ese campo como requerido: false (sectores_catalogo.go:346) — es decir, Hotel sin fecha genera un nodo inválido en vez de un error de validación.
10. paquetes.go y masiva.go son duplicados casi literales.
normalized(), validateBase(), sector(), codigoEmision(), extraerArchivo*() — ~150 líneas repetidas con la misma semántica y los mismos comentarios.
11. Ruido de depuración con datos sensibles (§11).
facturacion.go:739-740 (log.Println(resp) de toda la respuesta SIAT en cada operación), siat_usecase.go:267,306 (log.Printf volcando CUFD) y un bloque comentado con fragmentos base64 de CUFD en siat_usecase.go:240-247.
12. Multi-tenancy inexistente (documentado, fuera de alcance por decisión tomada).
siat.Service envuelve un único goSiat.SiatServices con NIT, codigoSistema, ambiente y certificado del .env, y applyIdentityValues (service.go:190) rechaza cualquier solicitud cuyo NIT no coincida. withDynamicConfig ignora sus argumentos por diseño. Supay es hoy mono-empresa de facto; el §7 y Supay Cloud requieren un Service (o Config) por empresa. Bloqueante conocido, no se toca en esta tanda.
go-siat
Ya lo resuelve — no reimplementar:
- Los 50 builders sectoriales, sus modelos y el codigoDocumentoSector por defecto en cada cabecera.
- Nilable[T] / TimeSiat: la semántica xsi:nil y el formato de fecha del SIAT son del SDK, correctos y testeados por él.
- Firma (Config().SignXML, XAdES-BES), utils.CompressAndHash (gzip+Base64+SHA-256), utils.NewCUF().
- WithFacturas: tar + firma + compresión + hash del lote.
- Las 12 fachadas SOAP y los builders de recepción/anulación/reversión/paquete/masiva.
- El catálogo normativo documentado, incluido qué sector va por qué fachada y el único código sin builder (33).
Supay duplica hoy (señalado según §3):
- extraerResultadoFacturacion y toMensajes reimplementan por reflexión la lectura de respuestas porque los tipos viven en paquetes internal/ del SDK. Es una limitación real del SDK (tipos no exportados) — se mantiene, envuelta donde está.
- La verificación manual Transaccion/CodigoEstado existe porque RespuestaRecepcion no implementa common.Result y goSiat.Verify no aplica. Reportar upstream: que las respuestas de recepción implementen common.Result.
Limitaciones reales a reportar upstream (§3), cada una con su workaround en un punto único y marcado:
1. Nilable[T] no puede omitir el nodo, solo emitirlo con xsi:nil. Supay necesita omitirlo (comportamiento homologado). → workaround: una única función de limpieza declarativa.
2. WithFacturas([]any, signer) serializa internamente y no acepta XML ya serializado, así que el lote no puede compartir el pipeline de la emisión individual. → pedir WithFacturasXML([][]byte); mientras tanto, empaquetar con WithArchivo/WithHashArchivo/WithCantidadFacturas (API pública ya usada para el sector 33), reutilizando SignXML y CompressAndHash del SDK.
3. Los builders sectoriales no comparten interfaz Go → justifica el aplicador reflexivo actual. Se conserva.
Propuesta
Mantener el registro sectorial y la reflexión. Cerrar las tres brechas que hacen que "50 sectores" sea nominal: campos de detalle, exhaustividad verificada y un solo pipeline de serialización. Validar en profundidad con dos sectores de referencia (1 CompraVenta ya homologado + 16 Hotel, que es el caso mínimo con campos sectoriales de cabecera y de detalle). Recién después extender sector por sector.
Paso 1 — Campos sectoriales de detalle (habilita los 50 sectores)
- SectorProfile: agregar CamposDetalle []CampoSector (misma estructura y validación que Campos).
- ItemFactura: agregar DatosSector json.RawMessage.
- Extraer de ValidarDatosSector un helper validarCampos(codigo int, campos []CampoSector, datos json.RawMessage) y usarlo para cabecera y para cada ítem (misma normalización, mismos mensajes de error, mismo rechazo de claves desconocidas).
- construirDetalle: después de los 9 comunes, aplicar CamposDetalle con tolerante=false (igual que la cabecera).
- Dominio/persistencia: domain.InvoiceItem.SectorData + columna sector_data jsonb en invoice_items (migración aditiva) + datos_sector en el ítem del request HTTP, validado en Create (fail-fast, antes de tocar la base), simétrico a lo que ya hace invoice_usecase.go:217.
- SectorAdapter no cambia: Prepare devuelve el SectorDocument con los valores de ítem ya normalizados.
Paso 2 — Exhaustividad verificada contra la superficie del SDK
Un helper único, fuente de verdad compartida por test y runtime:
camposNoCubiertos(p *SectorProfile) (cabecera []string, detalle []string)
Enumera por reflexión los métodos With* del builder de cabecera y de detalle del perfil, y resta: (a) los campos comunes de builder_reflex.go, (b) p.Campos / p.CamposDetalle, (c) una lista explícita de ignorados con motivo (WithCafc → contingencia; WithNumeroSerie/WithNumeroImei → opcionales del ítem; WithCodigoDocumentoSector → lo pone el perfil). Todo lo demás es una brecha.
- Test nuevo TestPerfilesCubrenLaSuperficieDelSDK: para cada perfil marcado Soportado, camposNoCubiertos debe estar vacío. Para el resto, imprime el inventario exacto de lo que falta (hoja de ruta ejecutable de los sectores restantes).
- Guarda en runtime en buildFacturaSDK: si el perfil no está Soportado, error explícito nombrando los campos faltantes. Es mejor un 400 de Supay que un documento fiscal incompleto emitido al SIAT.
- Soportado reemplaza a Experimental (que hoy solo significa "acepta datos_sector vacío" y no protege nada). Perfiles iniciales Soportado: 1, 8, 11, 24 (ambos layouts), 29, 46, 47, 48 — los que ya tienen casos aceptados — más 16 al terminar el Paso 4.
Paso 3 — Un solo pipeline documento → XML → firma → archivo
Función única en internal/siat:
serializarDocumento(doc any, modalidad int, cfg goSiat.Config) (xmlFirmado []byte, err error)
xml.Marshal → limpieza declarativa de nodos nilables → SignXML si electrónica. Y empaquetarFacturas([][]byte) (archivo, hash string, err error) que arma el tar, comprime y hashea con utils.CompressAndHash.
- EmitirFactura la usa (comportamiento idéntico al actual: misma lista de nodos, mismo orden).
- EnviarPaqueteFactura / EnviarMasivaFacturas la usan y setean WithArchivo/WithHashArchivo/WithCantidadFacturas en vez de WithFacturas. Un comentario // WORKAROUND go-siat#<issue> marca el punto único a remover cuando el SDK acepte XML pre-serializado.
- La lista de nodos a omitir pasa a ser declarativa y comentada (telefono, complemento, montoDescuentoCreditoDebito, cafc) en lugar de un literal enterrado en una función.
Paso 4 — CAFC como campo real y Hotel (16) como sector de referencia
- SolicitudFactura.Cafc *string + WithCafc de vuelta en los campos comunes: con valor → viaja el valor; sin valor → el nodo se omite (comportamiento homologado restaurado). Esto cumple el objetivo de 583c8c4 sin romper la homologación y devuelve HEAD a verde.
- Hotel: verificar cada campo contra el XSD del sector y declarar requerido en consecuencia (en particular fecha_ingreso_hospedaje, hoy false, que produce un nodo vacío inválido — Hallazgo 9). Agregar CamposDetalle: codigo_tipo_habitacion (int), detalle_huespedes (string). Marcar Soportado cuando el test de exhaustividad pase y el XML golden esté revisado campo a campo.
Paso 5 — Deuda de bajo riesgo (independiente, se puede hacer en paralelo)
- Unificar paquetes.go/masiva.go con un tipo embebido común (loteFacturas) para normalized/validateBase/sector/codigoEmision y un extraerArchivoLote(nodo string) parametrizado. ~150 líneas menos, cero cambio de comportamiento.
- Eliminar log.Println(resp) de extraerResultadoFacturacion, los log.Printf de CUFD y el bloque comentado con base64 en siat_usecase.go.
Lo que NO se hace en esta tanda
Reemplazar la reflexión por 50 adaptadores tipados (sería la reescritura que el §17 prohíbe, y la reflexión no es la causa de los fallos: la falta de declaración y de verificación sí lo es). Tampoco: implementar sectores nuevos más allá de Hotel, ni tocar multi-tenancy, ni recepción de compras.
Riesgo
Paso	Riesgo	Qué NO debe cambiar
1 Campos de detalle	Bajo	Un ítem sin datos_sector debe producir XML byte-idéntico al actual. Aditivo puro.
2 Exhaustividad	Bajo	Es test + guarda. Único efecto en runtime: sectores nunca homologados pasan de "emiten XML incompleto" a "error explícito". Ningún sector con casos aceptados puede quedar fuera de Soportado.
3 Pipeline único	Medio/Alto	Individual: bytes idénticos. Paquete/masiva: cambian a propósito (dejan de emitir xsi:nil).
4 CAFC + Hotel	Medio	Sin Cafc: XML idéntico al de HEAD~1 (nodo cafc ausente). Con Cafc: <cafc>VALOR</cafc>. Hotel es sector nuevo: no hay homologación que preservar.
5 Deuda	Bajo	Refactor puro con tests existentes verdes.
Detalle del Paso 3 (Medio/Alto). Es el único cambio que altera un documento fiscal ya enviado al SIAT.
Qué no debe cambiar: la emisión individual. El XML firmado, el archivo Base64 y el hashArchivo de EmitirFactura deben ser byte-idénticos a los de HEAD~1 para los casos de los fixtures. Cualquier diferencia, incluso de orden de atributos o whitespace, se reporta antes de continuar.
Qué cambia a propósito: el XML de cada factura dentro del tar de paquete y masiva pierde los nodos xsi:nil que hoy envía. Es la eliminación de la divergencia del Hallazgo 6, alineando el lote con la ruta que el SIAT ya acepta.
Cómo se comprueba: fixtures de paquete y masiva capturados antes del cambio; diff campo a campo del XML descomprimido del tar; el delta esperado es exactamente el conjunto de nodos de la lista de limpieza y nada más. La firma se recalcula sobre el XML limpio (igual que en la ruta individual), así que el hash cambia por definición: lo que se verifica es el contenido, no el hash.
Convivencia (§5): la lista de nodos a omitir queda en una única variable. Si el SIAT rechazara un paquete limpio, se revierte el lote a la ruta WithFacturas cambiando una línea, sin tocar la emisión individual. Dado que el ejemplo oficial del SIAT sí admite xsi:nil (Hallazgo 8), la hipótesis contraria queda comprobable con ese mismo interruptor. No se justifica un feature flag por empresa: el interruptor es global, de una línea y reversible.
Cambio
internal/siat/sectores.go
  + SectorProfile.CamposDetalle []CampoSector
  + SectorProfile.Soportado bool           (reemplaza Experimental)
  ~ ValidarDatosSector → extraer validarCampos(codigo, campos, datos)
  + ValidarDatosItem(item) usando validarCampos

internal/siat/facturacion.go
  + SolicitudFactura.Cafc *string
  + ItemFactura.DatosSector json.RawMessage
  + serializarDocumento(doc, modalidad, cfg)   ← pipeline único
  + empaquetarFacturas([][]byte)
  ~ EmitirFactura: usa serializarDocumento
  ~ removeEmptyOptionalFacturaFields → nodosOmitidosSinValor (var declarativa + comentario WORKAROUND)
  ~ buildFacturaSDK: guarda de perfil no Soportado con camposNoCubiertos
  - log.Println(resp) en extraerResultadoFacturacion

internal/siat/builder_reflex.go
  + {"WithCafc", req.Cafc} en los comunes
  ~ construirDetalle: aplicar p.CamposDetalle con tolerante=false
  + camposNoCubiertos(p) + camposComunesCabecera/Detalle + ignoradosConMotivo (fuente única)

internal/siat/sectores_catalogo.go
  ~ Hotel (16): requerido según XSD + CamposDetalle (codigo_tipo_habitacion, detalle_huespedes)
  ~ marcar Soportado: 1, 8, 11, 24 (x2), 29, 46, 47, 48 (+16 al cerrar el Paso 4)

internal/siat/paquetes.go, masiva.go
  ~ usar serializarDocumento + empaquetarFacturas en vez de WithFacturas
  ~ unificar normalized/validateBase/sector/codigoEmision/extraerArchivo en un tipo embebido

internal/domain/invoice.go, repository/postgres/invoice_repo.go, migrations/
  + InvoiceItem.SectorData + columna sector_data jsonb (aditiva)

internal/usecase/invoice_usecase.go
  + datos_sector por ítem en el request + validación fail-fast en Create
internal/usecase/emission.go
  + propagar DatosSector del ítem y Cafc a SolicitudFactura
internal/usecase/siat_usecase.go
  - log.Printf de CUFD y bloque comentado con base64
Verificación
Fixtures (capturar ANTES de tocar código, desde HEAD~1 = 583c8c4^, que es el último estado con la ruta individual verde):
internal/siat/testdata/ — XML desempaquetado y normalizado (sin firma) de: compraventa electrónica, compraventa computarizada, sector educativo 11, tasa cero 8, nota 24 (ambos layouts), nota 47, nota 48, paquete (factura1.xml del tar), masiva (factura1.xml del tar). Fixture externo de referencia normativa: facturaElectronicaCompraVenta.xml de la raíz (ejemplo oficial SIAT).
Regresión (§13) — el criterio de "terminado":
1. TestFixturesEmisionIndividual: para cada fixture individual, el XML generado es byte-idéntico. Falla ⇒ se detiene el trabajo y se reporta el diff, sea cosmético o no.
2. TestFixturesPaqueteMasiva: el diff contra el fixture debe consistir exclusivamente en la ausencia de los nodos de nodosOmitidosSinValor. Cualquier otro delta es un bug.
3. TestEmitirFacturaCompraVentaPayload y TestEmitirFacturaSectorEducativoPayload (hoy en rojo) vuelven a verde sin relajar sus aserciones.
4. Nuevo TestCafcViajaConValor: Cafc="ABC123" ⇒ <cafc>ABC123</cafc>; Cafc=nil ⇒ nodo ausente.
Unitarios nuevos:
5. TestPerfilesCubrenLaSuperficieDelSDK: todo perfil Soportado con cobertura 100%; el resto reporta su inventario de faltantes.
6. TestSectorNoSoportadoRechazaEmision: sector 19 ⇒ error que nombra montoIehd y porcentajeIehd, sin llegar al SIAT.
7. TestCamposDetalleSectorHotel: codigoTipoHabitacion y detalleHuespedes presentes por ítem; fecha_ingreso_hospedaje ausente ⇒ error de validación (no nodo vacío).
8. TestItemSinDatosSectorNoCambiaXML: prueba de aditividad del Paso 1.
Integración: los tests de httptest existentes de facturacion_test.go, paquetes_test.go y masiva_test.go deben seguir verdes, incluido TestEnviarPaqueteFacturaFirmadoPreservaXsi (la declaración xmlns:xsi se conserva; solo desaparecen los nodos xsi:nil).
Cierre: go test ./... verde (hoy no lo está) + inventario de camposNoCubiertos publicado como hoja de ruta de los sectores restantes, ordenado por cantidad de campos faltantes.
Dos cosas que quiero que confirmes antes de que Sonnet toque código:
1. El Paso 3 cambia el XML de paquete/masiva a propósito. Si tenés paquetes ya aceptados por el SIAT con el XML actual (con xsi:nil), decímelo: en ese caso los fixtures de paquete pasan a ser intocables y el pipeline único se aplica solo a la ruta individual, dejando la divergencia documentada en vez de resuelta.
2. Confirmame que puedo declarar Soportado exactamente para 1, 8, 11, 24, 29, 46, 47 y 48. Si algún otro sector ya tuvo casos aceptados por el SIAT, tiene que entrar en esa lista — si no, la guarda del Paso 2 te va a bloquear una emisión que hoy funciona.