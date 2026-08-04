import {
    Injectable,
    InternalServerErrorException,
} from '@nestjs/common';
import * as soap from 'soap';
import { PrismaService } from '../prisma/prisma.service';
import { ExceptionsHandler } from '@nestjs/core/exceptions/exceptions-handler';

@Injectable()
export class SiatService {

    constructor(private readonly prisma: PrismaService) { }

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


    async triggerCuis(companyId: string, pointOfSaleId: string) {
        try {
            const company = await this.prisma.company.findUnique({ where: { id: companyId } })
            const pointOfSale = await this.prisma.pointOfSale.findUnique({ where: { id: pointOfSaleId } })

            if (!company && !pointOfSale) throw new Error('No se puedo obtener compnay y pointOfSale para generar las CUIS')
            const client = await this.obtenerCliente();
            const request = {
                codigoAmbiente: this.ambiente,
                codigoModalidad: this.modalidad,
                codigoSistema: company?.codigoSistema,
                nit: company?.nit,
                codigoSucursal: pointOfSale?.codigoSucursal,
                codigoPuntoVenta: pointOfSale?.codigoPuntoVenta,
            };
            const [response] = await client.cuisAsync({
                SolicitudCuis: request,
            })
            let codeCuis = response.RespuestaCuis.codigo;
            console.log(response)

            const updatePointOfSale = await this.prisma.pointOfSale.update({ where: { id: pointOfSaleId }, data: { cuis: codeCuis, cuisCreatedAt: new Date() } })
            return { succcess: true, data: updatePointOfSale, response }
        }
        catch (error) {
            throw new InternalServerErrorException('No se pudo generar CUIS try later...')
        }
    }

    async createCufd(companyId: string, poinOfSaleId: string) {
        try {
            const company = await this.prisma.company.findUnique({where:{id:companyId},select:{nit:true,codigoSistema:true}})
            const pointOfSale = await this.prisma.pointOfSale.findUnique({where:{id:poinOfSaleId}, select:{codigoPuntoVenta:true,codigoSucursal:true,cuis:true}})
            const cliente = await this.obtenerCliente();
            const solicitud = {
                codigoAmbiente: this.ambiente,
                codigoModalidad: this.modalidad,
                codigoSistema: company?.codigoSistema,
                nit: company?.nit,
                codigoSucursal: pointOfSale?.codigoSucursal,
                codigoPuntoVenta: pointOfSale?.codigoPuntoVenta,
                cuis:pointOfSale?.cuis,
            };

            const [respuesta] = await cliente.cufdAsync({
                SolicitudCufd: solicitud,
            });
            const dat = {
                 pointOfSaleId:poinOfSaleId,
                    codigoControl:respuesta.RespuestaCufd.codigoControl,
                    direccion:respuesta.RespuestaCufd.direccion,
                    validFrom : new Date(),
                    cufd:respuesta.RespuestaCufd.codigo,
                    validTo: respuesta.RespuestaCufd.fechaVigencia
            }
            return {respuesta,dat}
        } catch (error) {
            console.error('Error obteniendo CUFD:', error);
            throw new InternalServerErrorException(
                'No se pudo obtener el CUFD del SIAT',
            );
        }
    }
}