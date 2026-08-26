import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  vus: 1,
  iterations: 10,
};
const BASE_URL = 'http://localhost:8081';
const API_KEY = 'efc4c5d72e7c272881ffdaef03328a7ef2fbad363f4de23af10d741ca2f1fffb';

const COMPANY_ID = '1da4cdec-32c5-4f32-bfb6-ebc3a6ef1b7f';
const POS_ID = 'a3d89d57-03bb-4b86-9771-879eee79e415'; 

// IMPORTANTE: Asegúrate de usar un CUFD activo que se haya creado ANTES de las 16:35
const CUFD_VIGENTE = 'FBQUtCV1DDgUpBQ4NzM5NDA4QUI2QkFfUkNRWUlhVUMjI4NDUyQzM4RU"';

const headers = {
  'Content-Type': 'application/json',
  'X-API-Key': API_KEY,
};
function getSequenceDate(iter, isEnd) {
  // Punto de partida fijo: 16:35:00
  const baseTime = new Date('2026-08-23T17:10:00.000Z');
  
  // Avanzamos 1 minuto por cada iteración
  baseTime.setMinutes(baseTime.getMinutes() + iter);
  
  if (isEnd) {
      // El fin siempre es en el segundo 59 del mismo minuto
      baseTime.setSeconds(59);
  } else {
      // El inicio siempre es en el segundo 00
      baseTime.setSeconds(0);
  }
  
  return baseTime.toISOString().replace('Z', ''); 
}

function registerSignificantEvent(iter) {
  // 1. Armar el payload
  const payload = JSON.stringify({
    codigoMotivoEvento: 4, 
    descripcion: "VENTA EN LUGARES SIN INTERNET",
    cufdEvento: CUFD_VIGENTE,
    fechaHoraInicioEvento: getSequenceDate(iter, false), 
    fechaHoraFinEvento: getSequenceDate(iter, true)
  });

  const url = `${BASE_URL}/siat/evento-significativo/${COMPANY_ID}/${POS_ID}`;
  
  // 2. Enviar petición
  const res = http.post(url, payload, { headers });
  
  // 3. Validar éxito
  const success = check(res, {
    [`evento ${iter + 1} status 200/201`]: (r) => r.status === 200 || r.status === 201,
    [`evento ${iter + 1} tx exitosa`]: (r) => {
        try {
            return r.json('response.transaccion') === true;
        } catch (e) {
            return false;
        }
    },
  });

  if (!success) {
    console.error(`Error en Iteración ${iter + 1} | Fechas enviadas: Inicio=${getSequenceDate(iter, false)}, Fin=${getSequenceDate(iter, true)} \nRespuesta SIAT: ${res.body}`);
  } else {
    console.log(`✅ Evento ${iter + 1} registrado con éxito (${getSequenceDate(iter, false)} - ${getSequenceDate(iter, true)})`);
  }
}

export default function () {
  // k6 provee __ITER (empezando desde 0 hasta 9)
  registerSignificantEvent(__ITER);
  
  // Pausa exacta de 1 segundo entre peticiones para darle respiro a tu backend y al SIAT
  sleep(1);
}