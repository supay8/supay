import { Controller, Get, Param, Post, Query } from '@nestjs/common';
import { SiatService } from './siat.service';

@Controller('siat')
export class SiatController {
  constructor(private readonly siatService: SiatService) {}
 
  @Get('comunicacion')
  verificarComunicacion() {
    return this.siatService.verificarComunicacion();
  }

  @Post('cuis/:companyId/:poinOfSaleId')
  obtenerCuis(@Param('companyId') companyId:string,@Param('poinOfSaleId') pointOfSaleId) {
    console.log(companyId,pointOfSaleId)
    return this.siatService.triggerCuis(companyId,pointOfSaleId);
  }

  @Post('cufd/:companyId/:poinOfSaleId')
  obtenerCufd(@Param('companyId') companyId:string,@Param('poinOfSaleId') pointOfSaleId) {
    console.log(companyId,pointOfSaleId)
    if(!companyId || !pointOfSaleId ) return {message:'Paramas incompletos'}
    return this.siatService.createCufd(companyId,pointOfSaleId)
  }
}