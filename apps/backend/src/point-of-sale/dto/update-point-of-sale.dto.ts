import { PartialType } from '@nestjs/swagger';
import { CreatePointOfSaleDto } from './create-point-of-sale.dto';

export class UpdatePointOfSaleDto extends PartialType(CreatePointOfSaleDto) {}
