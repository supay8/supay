import { Module } from '@nestjs/common';
import { AppController } from './app.controller';
import { AppService } from './app.service';
import { SiatModule } from './siat/siat.module';
import { PrismaService } from './prisma/prisma.service';
import { PrismaModule } from './prisma/prisma.module';
import { CompanyModule } from './company/company.module';
import { PointOfSaleModule } from './point-of-sale/point-of-sale.module';

@Module({
  imports: [ SiatModule, PrismaModule, CompanyModule, PointOfSaleModule],
  controllers: [AppController],
  providers: [AppService, PrismaService],
})
export class AppModule {}
