import { Controller, Get, Query } from '@nestjs/common';
import { SiatService } from './siat.service';

@Controller('siat')
export class SiatController {
  constructor(private readonly siatService: SiatService) {}
 
  @Get('comunicacion')
  verificarComunicacion() {
    return this.siatService.verificarComunicacion();
  }

  @Get('cuis')
  obtenerCuis() {
    return this.siatService.obtenerCuis();
  }

  @Get('cufd')
  obtenerCufd(@Query('cuis') cuis: string) {
    return this.siatService.obtenerCufd(cuis);
  }
}