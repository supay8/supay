# Observabilidad

Supay expone métricas Prometheus en `GET /metrics` y `GET /v1/metrics`. El
endpoint no requiere credenciales y debe publicarse solamente en la red interna
del despliegue.

## Métricas

| Métrica | Uso |
| --- | --- |
| `supay_http_requests_total` | Volumen por método, ruta normalizada y status HTTP. |
| `supay_http_request_duration_seconds` | Histograma de latencia por método y ruta. |
| `supay_http_requests_in_flight` | Solicitudes que se están atendiendo. |
| `supay_invoice_emissions_total` | Intentos de emisión por resultado acotado. |
| `supay_invoice_emission_duration_seconds` | Histograma de duración de emisiones. |
| `supay_outbox_events_total` | Publicación y errores del outbox. |

Las etiquetas nunca contienen tenants, NIT, facturas, clientes, productos ni
rutas concretas con IDs. Esto evita tanto PII como cardinalidad no acotada.

## Prometheus y Grafana

`deploy/observability/prometheus.yml` incluye un scrape de ejemplo. Grafana
puede aprovisionar el datasource y el dashboard desde
`deploy/observability/grafana/provisioning` y
`deploy/observability/grafana/dashboards/supay-overview.json`.

Alertas iniciales recomendadas:

- tasa de 5xx mayor a 2% durante 10 minutos;
- emisiones con `result="retry"` o `result="circuit_open"` sostenidas;
- ausencia de `result="published"` en outbox mientras hay tráfico;
- p95 HTTP o de emisión por encima del SLO acordado.

## Logs

Los logs son JSON por defecto (`LOG_FORMAT=text` habilita texto local). Cada
solicitud registra `request_id`, método, patrón de ruta, status y duración. El
handler global redacta credenciales y PII, reemplaza identificadores por hashes
cortos correlacionables y registra el tipo —no el contenido— de los errores.
No se deben interpolar payloads, query strings ni datos personales en el mensaje
del log.

## Errores HTTP

Todos los errores incluyen `code`, `message`, `sugerencias` y `accion`; cuando
se puede identificar con seguridad, también `field`. `details` conserva las
observaciones de rechazo del SIAT e `invoice_id` aparece cuando ya existe una
factura. Los consumidores deben programar contra `code`, no contra `message`.
