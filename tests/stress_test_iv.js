import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  stages: [
    { duration: '20s', target: 10 },
    { duration: '20s', target: 50 },
    { duration: '10s', target: 0 },
  ],
  thresholds: {
    http_req_failed: ['rate<0.05'],
    http_req_duration: ['p(95)<800'],
  },
};

const BASE_URL = 'http://localhost:8081';
const API_KEY = 'efc4c5d72e7c272881ffdaef03328a7ef2fbad363f4de23af10d741ca2f1fffb';

const COMPANY_ID = '1da4cdec-32c5-4f32-bfb6-ebc3a6ef1b7f';
const POS_ID = ['a3d89d57-03bb-4b86-9771-879eee79e415','88a078de-ab11-4985-b8fa-ab321b0254af'];

const CUSTOMER_ID = 'aaa0299d-033d-45a9-9782-833197a71e83';
const ORIGINAL_INVOICE_CUF = '7f020ea6-6a25-4cd4-9587-79c2cc4bbadb';

const headers = {
  'Content-Type': 'application/json',
  'X-API-Key': API_KEY,
};

function createAndEmitCreditNote(index) {
  // 1. Crear la Nota de Crédito-Débito (Sector 24)
  const createPayload = JSON.stringify({
    company_id: COMPANY_ID,
    point_of_sale_id: POS_ID[1],
    customer_id: CUSTOMER_ID,
    invoice_type: 'credit_note',
    codigo_documento_sector: 1,
  modalidad: 1,
  codigo_metodo_pago: 1,
  codigo_moneda: 1,
  tipo_cambio: 1,
  codigo_tipo_factura: 1,
  items: [
    {
      code: "PROD-001",
      description: "Consultoría en pedagogía - Parte 1",
      codigo_actividad: "8550100",
      codigo_producto_sin: "1004411",
      unit_code: 1,
      quantity: 1,
      unit_price: 1250,
      discount: 0
    }
  ]
  });

  const createRes = http.post(`${BASE_URL}/invoices/`, createPayload, { headers });
  const createSuccess = check(createRes, {
    [`nota de crédito ${index} creada con status 201`]: (r) => r.status === 201,
  });

  if (!createSuccess) {
    console.error(`Error creando nota ${index}:`, createRes.body);
    return;
  }

  const invoiceId = createRes.json('id');

  // 2. Emitir la nota de crédito-débito al SIAT
  const emitRes = http.post(`${BASE_URL}/invoices/${invoiceId}/emit`, null, { headers });
  check(emitRes, {
    [`nota de crédito ${index} emitida con status 200`]: (r) => r.status === 200,
    [`nota de crédito ${index} aceptada por SIAT`]: (r) => r.json('status') === 'ACCEPTED',
  });

  if (emitRes.status !== 200) {
    console.error(`Error emitiendo nota ${index} al SIAT:`, emitRes.body);
  }
}

export default function () {
  createAndEmitCreditNote(1);
  sleep(0.05);
}