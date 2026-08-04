import { Module } from '@nestjs/common';
import { PointOfSaleService } from './point-of-sale.service';
import { PointOfSaleController } from './point-of-sale.controller';
import { PrismaModule } from '../prisma/prisma.module';

@Module({
  imports:[PrismaModule],
  controllers: [PointOfSaleController],
  providers: [PointOfSaleService],
})
export class PointOfSaleModule {}
