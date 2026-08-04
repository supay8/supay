import { Module } from '@nestjs/common';
import { SiatService } from './siat.service';
import { SiatController } from './siat.controller';
import { PrismaModule } from '../prisma/prisma.module';

@Module({
  imports: [PrismaModule],
  providers: [SiatService],
  controllers: [SiatController],
  exports:[SiatService]
})
export class SiatModule {}
