import http from 'k6/http';
import { check, group, sleep } from 'k6';
import { Trend, Counter, Rate } from 'k6/metrics';
import { randomIntBetween, randomItem } from 'https://jslib.k6.io/k6-utils/1.2.0/index.js';
import { textSummary } from 'https://jslib.k6.io/k6-summary/0.0.2/index.js';

const BASE_URL = 'http://localhost:8081';
const API_KEY = 'sup_live_f48b5059_d9452c6523e147d4af36dd4d16a12e47';
const COMPANY_ID = __ENV.COMPANY_ID || 'de84156e-59ca-4381-94f7-2c7a34053320';
const POS_IDS = ["5226da47-74da-4b27-a26d-892c62fa6d26","4cdd19a8-ee70-4c90-8962-76f16b088508"]

const headers = {
  'Content-Type': 'application/json',
  'X-API-Key': API_KEY,
};

// ---------------------------------------------------------------------------
// Métricas personalizadas — separan la fase de creación de la de emisión SIAT,
// que suelen tener latencias muy distintas.
// ---------------------------------------------------------------------------
const createDuration = new Trend('nc_create_duration', true);
const emitDuration = new Trend('nc_emit_duration', true);
const siatAcceptedRate = new Rate('siat_accepted_rate');
const createErrors = new Counter('nc_create_errors');
const emitErrors = new Counter('nc_emit_errors');

// ---------------------------------------------------------------------------
// Catálogo de productos de prueba — cada factura toma una combinación aleatoria
// de 1 a 5 items, en vez de repetir siempre los mismos dos.
// ---------------------------------------------------------------------------
const PRODUCT_CATALOG = [
  { code: 'PROD-001', description: 'Consultoría en pedagogía - Parte 1', codigo_actividad: '8550100', codigo_producto_sin: '1004411', unit_code: 1, unit_price: 1250 },
  { code: 'PROD-002', description: 'Consultoría en pedagogía - Parte 2', codigo_actividad: '8550100', codigo_producto_sin: '1004411', unit_code: 1, unit_price: 1250 },
  { code: 'PROD-003', description: 'Material didáctico impreso', codigo_actividad: '8549100', codigo_producto_sin: '1004386', unit_code: 1, unit_price: 85 },
  { code: 'PROD-004', description: 'Licencia de plataforma virtual (mensual)', codigo_actividad: '8549100', codigo_producto_sin: '1004385', unit_code: 1, unit_price: 400 },
  { code: 'PROD-005', description: 'Soporte técnico especializado', codigo_actividad: '8549100', codigo_producto_sin: '1004387', unit_code: 1, unit_price: 300 },
];

const CUSTOMERS = [
  {
    client_document_type: "CI",
    client_document_number: "9971522",
    client_name: "BRandon",
    client_email: "ramitpr53@gmail.com"
  },
  {
    client_document_type: "CI",
    client_document_number: "4829103",
    client_name: "Valeria Gomez",
    client_email: "valeria.gomez@gmail.com"
  },
  {
    client_document_type: "NIT",
    client_document_number: "1028394019",
    client_name: "Inversiones Tech S.R.L.",
    client_email: "contacto@techstore.bo"
  },
  {
    client_document_type: "CI",
    client_document_number: "6371829",
    client_name: "Alejandro Mamani",
    client_email: "alex.mamani@outlook.com"
  },
  {
    client_document_type: "CI",
    client_document_number: "7182934",
    client_name: "Camila Quispe",
    client_email: "camila.quispe@gmail.com"
  },
  {
    client_document_type: "NIT",
    client_document_number: "3948572018",
    client_name: "Comercializadora Andina S.A.",
    client_email: "facturacion@andina.bo"
  }
];


const PAYMENT_METHODS = [1, 2, 3];
const CURRENCIES = [1, 2];
const MODALIDADES = [1, 2];

function buildRandomItems() {
  const itemCount = randomIntBetween(1, 5);
  const items = [];
  for (let i = 0; i < itemCount; i++) {
    const product = randomItem(PRODUCT_CATALOG);
    items.push({
      code: product.code,
      description: product.description,
      codigo_actividad: product.codigo_actividad,
      codigo_producto_sin: product.codigo_producto_sin,
      unit_code: product.unit_code,
      quantity: randomIntBetween(1, 10),
      unit_price: product.unit_price,
      discount: randomItem([0, 0, 0, 5, 10]), // mayoría sin descuento, algunos con
    });
  }
  return items;
}

function createAndEmitCreditNote(vu, iter) {
  const tag = `vu${vu}-iter${iter}`;
  const randomIndex = Math.floor(Math.random() * POS_IDS.length);
  const createPayload = JSON.stringify({
    company_id: COMPANY_ID,
    point_of_sale_id: POS_IDS[randomIndex],
    ...CUSTOMERS[randomIndex],
    codigo_documento_sector: 1,
    codigo_tipo_factura: 1,
    modalidad: 1,
    codigo_metodo_pago: 1,
    codigo_moneda: 1,
    tipo_cambio: 1,
    items: buildRandomItems(),
  });

  let invoiceId;

  group('Crear factura de compra-venta', function () {
    const createRes = http.post(`${BASE_URL}/invoices/`, createPayload, {
      headers,
    });
    createDuration.add(createRes.timings.duration);

    const createSuccess = check(createRes, {
      [`[${tag}] creada con status 201`]: (r) => r.status === 201,
      [`[${tag}] respuesta trae id`]: (r) => !!r.json('id'),
    });

    if (!createSuccess) {
      createErrors.add(1);
      console.error(`[${tag}] Error creando factura:`, createRes.status, createRes.body);
      return;
    }

    invoiceId = createRes.json('id');
  });

  if (!invoiceId) return;

  group('Emitir factura al SIAT', function () {
    const emitRes = http.post(`${BASE_URL}/invoices/${invoiceId}/emit`, null, {
      headers,
    });
    emitDuration.add(emitRes.timings.duration);

    const emitOk = emitRes.status === 200;
    const accepted = emitOk && emitRes.json('status') === 'ACCEPTED';
    siatAcceptedRate.add(accepted);

    check(emitRes, {
      [`[${tag}] emitida con status 200`]: () => emitOk,
      [`[${tag}] aceptada por SIAT`]: () => accepted,
    });

    if (!emitOk) {
      emitErrors.add(1);
      console.error(`[${tag}] Error emitiendo la factura al SIAT:`, emitRes.status, emitRes.body);
    }
  });
  group('Anular factura al SIAT', function () {
    const payload = JSON.stringify({
      codigo_motivo:1
    })
    const annulRes = http.post(`${BASE_URL}/invoices/${invoiceId}/annul`, payload, {
      headers,
      
    });
    emitDuration.add(annulRes.timings.duration);

    const annulOk = annulRes.status === 200;
    const accepted = annulOk && annulRes.json('status') === 'ACCEPTED';
    siatAcceptedRate.add(accepted);

    check(annulRes, {
      [`[${tag}] emitida con status 200`]: () => annulOk,
      [`[${tag}] aceptada por SIAT`]: () => accepted,
    });

    if (!annulOk) {
      emitErrors.add(1);
      console.error(`[${tag}] Error emitiendo la factura al SIAT:`, annulRes.status, annulRes.body);
    }
  });

  group('Revertir factura al SIAT', function () {
    const payload = JSON.stringify({
      codigo_motivo:1
    })
    const revertRes = http.post(`${BASE_URL}/invoices/${invoiceId}/annul/revert`, null, {
      headers,
      
    });
    emitDuration.add(revertRes.timings.duration);

    const reverOk = revertRes.status === 200;
    const accepted = reverOk && revertRes.json('status') === 'ACCEPTED';
    siatAcceptedRate.add(accepted);

    check(revertRes, {
      [`[${tag}] emitida con status 200`]: () => reverOk,
      [`[${tag}] aceptada por SIAT`]: () => accepted,
    });

    if (!reverOk) {
      emitErrors.add(1);
      console.error(`[${tag}] Error emitiendo la factura al SIAT:`, revertRes.status, revertRes.body);
    }
  });
}

// ---------------------------------------------------------------------------
// Escenarios de carga. Elige cuál correr con -e K6_SCENARIO=smoke|load|stress|spike
// ---------------------------------------------------------------------------
const SCENARIO = __ENV.K6_SCENARIO || 'load';

const scenarios = {
  smoke: {
    executor: 'per-vu-iterations',
    vus: 1,
    iterations: 5,
    maxDuration: '1m',
  },
  load: {
    executor: 'ramping-vus',
    startVUs: 0,
    stages: [
      { duration: '30s', target: 20 },
      { duration: '1m', target: 20 },
      { duration: '30s', target: 0 },
    ],
    gracefulRampDown: '10s',
  },
  stress: {
    executor: 'ramping-vus',
    startVUs: 0,
    stages: [
      { duration: '30s', target: 20 },
      { duration: '1m', target: 50 },
      { duration: '1m', target: 100 },
      { duration: '30s', target: 0 },
    ],
    gracefulRampDown: '10s',
  },
  spike: {
    executor: 'ramping-vus',
    startVUs: 0,
    stages: [
      { duration: '10s', target: 5 },
      { duration: '10s', target: 150 },
      { duration: '30s', target: 150 },
      { duration: '20s', target: 0 },
    ],
    gracefulRampDown: '10s',
  },
};
export const options = {
  scenarios: {
    [SCENARIO]: scenarios[SCENARIO],
  },
  thresholds: {
    http_req_failed: ['rate<0.05'],
    http_req_duration: ['p(95)<2000'],
    'http_req_duration{name:CreateCreditNote}': ['p(95)<1500'],
    'http_req_duration{name:EmitCreditNote}': ['p(95)<3000'], // SIAT suele ser más lento
    siat_accepted_rate: ['rate>0.95'],
  },
};
 

export default function () {
  createAndEmitCreditNote(__VU, __ITER);
  sleep(randomIntBetween(1, 3) / 10); // 0.1s–0.3s, simula pacing real de usuarios
}

export function handleSummary(data) {
  return {
    stdout: textSummary(data, { indent: ' ', enableColors: true }),
    'summary.json': JSON.stringify(data, null, 2),
  };
}