import { Controller, Get, Post, Body, Patch, Param, Delete } from '@nestjs/common';
import { PointOfSaleService } from './point-of-sale.service';
import { CreatePointOfSaleDto } from './dto/create-point-of-sale.dto';
import { UpdatePointOfSaleDto } from './dto/update-point-of-sale.dto';

@Controller('point-of-sale')
export class PointOfSaleController {
  constructor(private readonly pointOfSaleService: PointOfSaleService) {}

  @Post()
  create(@Body() createPointOfSaleDto: CreatePointOfSaleDto) {
    return this.pointOfSaleService.create(createPointOfSaleDto);
  }

  @Get()
  findAll() {
    return this.pointOfSaleService.findAll();
  }

  @Get(':id')
  findOne(@Param('id') id: string) {
    return this.pointOfSaleService.findOne(+id);
  }

  @Patch(':id')
  update(@Param('id') id: string, @Body() updatePointOfSaleDto: UpdatePointOfSaleDto) {
    return this.pointOfSaleService.update(+id, updatePointOfSaleDto);
  }

  @Delete(':id')
  remove(@Param('id') id: string) {
    return this.pointOfSaleService.remove(+id);
  }
}
