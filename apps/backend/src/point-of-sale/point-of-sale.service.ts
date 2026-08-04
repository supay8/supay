import { Injectable } from '@nestjs/common';
import { CreatePointOfSaleDto } from './dto/create-point-of-sale.dto';
import { UpdatePointOfSaleDto } from './dto/update-point-of-sale.dto';

@Injectable()
export class PointOfSaleService {
  create(createPointOfSaleDto: CreatePointOfSaleDto) {
    return 'This action adds a new pointOfSale';
  }

  findAll() {
    return `This action returns all pointOfSale`;
  }

  findOne(id: number) {
    return `This action returns a #${id} pointOfSale`;
  }

  update(id: number, updatePointOfSaleDto: UpdatePointOfSaleDto) {
    return `This action updates a #${id} pointOfSale`;
  }

  remove(id: number) {
    return `This action removes a #${id} pointOfSale`;
  }
}
