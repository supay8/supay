import http from 'k6/http';
import { check, sleep } from 'k6';

// Configuración de la rampa de estrés
export const options = {
  stages: [
    { duration: '20s', target: 10 },  // Sube gradualmente a 10 usuarios concurrentes
    { duration: '20s', target: 50 }, // Pico de estrés con 100 usuarios concurrentes
    { duration: '20s', target: 100 }, // Pico de estrés con 100 usuarios concurrentes
    { duration: '10s', target: 0 },   // Enfriamiento
  ],
  thresholds: {
    http_req_failed: ['rate<0.05'],   // Menos del 5% de errores permitidos
    http_req_duration: ['p(95)<500'], // El 95% de las peticiones deben responder en menos de 500ms
  },
};

export default function () {
  // Ajusta tu baseUrl, companyId y pointOfSaleId aquí:
  const BASE_URL = 'http://localhost:8081';
  const COMPANY_ID = '4732a5d8-dae1-45ee-b86d-618311b7725e';
  const POS_ID = '157a7c7e-2985-4847-938c-d306ba6b026f';

  const url = `${BASE_URL}/siat/cufd/${COMPANY_ID}/${POS_ID}`;

  const payload = JSON.stringify({});

  const params = {
    headers: {
      'Content-Type': 'application/json',
      'X-API-Key': 'efc4c5d72e7c272881ffdaef03328a7ef2fbad363f4de23af10d741ca2f1fffb',
    },
  };

  const res = http.post(url, payload, params);

  // Validar que el servidor responda con estado HTTP exitoso
  check(res, {
    'status es 200 o 201': (r) => r.status === 200 || r.status === 201,
  });

  sleep(0.05); // Pausa breve entre peticiones de cada usuario virtual
}