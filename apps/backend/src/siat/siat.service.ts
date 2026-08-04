import {
    Injectable,
    InternalServerErrorException,
} from '@nestjs/common';
import * as soap from 'soap';

@Injectable()
export class SiatService {
    private readonly wsdlUrl =
        'https://pilotosiatservicios.impuestos.gob.bo/v2/FacturacionCodigos?wsdl';

    private readonly endpoint =
        'https://pilotosiatservicios.impuestos.gob.bo/v2/FacturacionCodigos';

    private readonly ambiente = Number(process.env.SIAT_AMBIENTE ?? 2);
    private readonly modalidad = Number(process.env.SIAT_MODALIDAD ?? 2);
    private readonly nit = Number(process.env.SIAT_NIT);
    private readonly codigoSistema = process.env.SIAT_CODIGO_SISTEMA;
    private readonly codigoSucursal = Number(
        process.env.SIAT_CODIGO_SUCURSAL ?? 0,
    );
    private readonly codigoPuntoVenta = Number(
        process.env.SIAT_CODIGO_PUNTO_VENTA ?? 0,
    );
    private readonly tokenDelegado = process.env.SIAT_TOKEN_DELEGADO;

    private async obtenerCliente(): Promise<soap.Client> {
        if (!this.tokenDelegado) {
            throw new Error('Falta SIAT_TOKEN_DELEGADO');
        }
        if (!this.codigoSistema) {
            throw new Error('Falta SIAT_CODIGO_SISTEMA');
        }
        const cliente = await soap.createClientAsync(this.wsdlUrl);

        // Endpoint real: sin ?wsdl
        cliente.setEndpoint(this.endpoint);

        // Formato actual del Token Delegado para estos servicios
        cliente.addHttpHeader(
            'apikey', `TokenApi ${this.tokenDelegado}`,
        );

        return cliente;
    }

    async verificarComunicacion() {
        const cliente = await this.obtenerCliente();
        const [respuesta] = await cliente.verificarComunicacionAsync({});
        return respuesta;
    }



    async obtenerCuis() {
        const cliente = await this.obtenerCliente();

        const solicitud = {
            codigoAmbiente: this.ambiente,
            codigoModalidad: this.modalidad,
            codigoSistema: this.codigoSistema,
            nit: this.nit,
            codigoSucursal: this.codigoSucursal,
            codigoPuntoVenta: this.codigoPuntoVenta,
        };
        console.log(solicitud)

        try {
            const [respuesta] = await cliente.cuisAsync({
                SolicitudCuis: solicitud,
            });

            return respuesta;
        } catch (error: any) {
            console.error('Error obteniendo CUIS');

            if (error?.response?.data) {
                console.error(error.response.data);
            } else {
                console.error(error);
            }

            throw new InternalServerErrorException(
                'No se pudo obtener el CUIS del SIAT',
            );
        }
    }
    async obtenerCufd(cuis: string) {
        const cliente = await this.obtenerCliente();
        console.log("obteniendo cuis")
        console.log(cuis)
        const solicitud = {
            codigoAmbiente: this.ambiente,
            codigoModalidad: this.modalidad,
            codigoSistema: this.codigoSistema,
            nit: this.nit,
            codigoSucursal: this.codigoSucursal,
            codigoPuntoVenta: this.codigoPuntoVenta,
            cuis,
        };

        try {
            const [respuesta] = await cliente.cufdAsync({
                SolicitudCufd: solicitud,
            });

            return respuesta;
        } catch (error) {
            console.error('Error obteniendo CUFD:', error);
            throw new InternalServerErrorException(
                'No se pudo obtener el CUFD del SIAT',
            );
        }
    }
}