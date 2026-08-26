import http from 'k6/http';
import { check, sleep } from 'k6';

// MEJORA: Para pasar la certificación, basta con 1 disparo perfecto. 
// Evitamos que tu PC se quede sin RAM renderizando XMLs.
export const options = {
  vus: 1,
  iterations: 10, 
  thresholds: {
    http_req_failed: ['rate<0.05'],
    // MEJORA: Tu backend en Go va a necesitar varios segundos para procesar 
    // y comprimir 1000 XMLs. Le damos 30 segundos de paciencia a k6.
    http_req_duration: ['p(95)<30000'], 
  },
};

const BASE_URL = 'http://localhost:8081';
const API_KEY = 'efc4c5d72e7c272881ffdaef03328a7ef2fbad363f4de23af10d741ca2f1fffb';

const COMPANY_ID = '1da4cdec-32c5-4f32-bfb6-ebc3a6ef1b7f';
const POS_ID = 'a3d89d57-03bb-4b86-9771-879eee79e415'; 

const headers = {
  'Content-Type': 'application/json',
  'X-API-Key': API_KEY,
};

// MEJORA: Restamos de a 1 SEGUNDO por cada factura. 
// 1000 facturas equivalen a ~16 minutos de tiempo. Todas recientes y sin chocar.
function getFormattedDate(i) {
  const d = new Date();
  d.setSeconds(d.getSeconds() - i);
  return d.toISOString().replace('Z', '-04:00');
}

function buildMasivaPayload() {
  const facturas = [];
  
  // EL DATO CLAVE DEL SIAT: Exigen exactamente 1000 facturas para este caso de prueba.
  const CANTIDAD_FACTURAS_POR_PAQUETE = 1000; 

  console.log(`⏳ Empaquetando ${CANTIDAD_FACTURAS_POR_PAQUETE} facturas en el JSON...`);

  for (let i = 1; i <= CANTIDAD_FACTURAS_POR_PAQUETE; i++) {
    // Numeración limpia: del 1001 al 2000
    const numFactura = 1000 + (__ITER * 1000) + i;

    facturas.push({
      "numeroFactura": numFactura,
      "fechaEmision": getFormattedDate(i), 
      "usuario": "SUPAY",
      "leyenda": "Ley N° 453: Puedes acceder a la reclamación cuando tus derechos han sido vulnerados.",
      "razonSocialEmisor": "EMPRESA TEST SRL",
      "municipio": "LA PAZ",
      "direccion": "AV. MOCK 123",
      "codigoMetodoPago": 1,
      "codigoMoneda": 1,
      "tipoCambio": 1,
      "montoTotal": 100,
      
      // NOTA: Como es Facturación Masiva (codigoEmision: 3), NO lleva CAFC.
      // (Tu backend de Go ya aprendió a omitirlo)

      "cliente": {
        "razonSocial": "CLIENTE TEST",
        "codigoTipoDocumentoIdentidad": 1,
        "numeroDocumento": "1234567",
        "codigoCliente": "C-001"
      },
      "items": [
        {
          "actividadEconomica": "8549100",
          "codigoProductoSin": 1004387,
          "codigoProducto": "P-001",
          "descripcion": `Producto Masivo ${numFactura}`,
          "cantidad": 1,
          "unidadMedida": 1,
          "precioUnitario": 100,
          "subTotal": 100
        }
      ]
    });
  }

  return JSON.stringify({
    "codigoEmision": 3,
    "facturas": facturas
  });
}

export default function () {
  const payload = buildMasivaPayload();
  const url = `${BASE_URL}/siat/masiva/${COMPANY_ID}/${POS_ID}`;
  
  console.log(`🚀 Enviando megapaquete al backend en Go (esto puede tardar unos segundos)...`);
  
  // Como el payload es gigante, a k6 le tomará un momentito enviarlo por POST
  const res = http.post(url, payload, { headers });
  
  const success = check(res, {
    [`Paquete 1000 facturas procesado con status 200/201`]: (r) => r.status === 200 || r.status === 201,
  });

  if (!success) {
    console.error(`❌ Error | Status: ${res.status} | Respuesta Go: ${res.body}`);
  } else {
    console.log(`✅ ¡ÉXITO! Go respondió con Status ${res.status}.`);
  }

  sleep(1);
}