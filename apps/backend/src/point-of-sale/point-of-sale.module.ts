import { Module } from '@nestjs/common';
import { PointOfSaleService } from './point-of-sale.service';
import { PointOfSaleController } from './point-of-sale.controller';

@Module({
  controllers: [PointOfSaleController],
  providers: [PointOfSaleService],
})
export class PointOfSaleModule {}
