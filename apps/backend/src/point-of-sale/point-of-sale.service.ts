import { 
  Injectable, 
  InternalServerErrorException, 
  NotFoundException 
} from '@nestjs/common';
import { CreatePointOfSaleDto } from './dto/create-point-of-sale.dto';
import { UpdatePointOfSaleDto } from './dto/update-point-of-sale.dto';
import { PointOfSale } from './entities/point-of-sale.entity';
import { PrismaService } from '../prisma/prisma.service';

@Injectable()
export class PointOfSaleService {
  constructor(private readonly prisma: PrismaService) {}
  
  async create(createPointOfSaleDto: CreatePointOfSaleDto) {
    try {
      const pointOfSales: PointOfSale[] = await this.prisma.pointOfSale.findMany({where:{companyId:createPointOfSaleDto.companyId}})
      let endCodePointOfSale = pointOfSales.length + 1;      
      const pointOfSale = await this.prisma.pointOfSale.create({
        data:{
          ...createPointOfSaleDto,
          codigoPuntoVenta:endCodePointOfSale
        }})
      return { success: true, data: pointOfSale };
    } catch (error) {
      console.error('Error al crear el punto de venta:', error);
      throw new InternalServerErrorException('Error creating point-of-sale, try later');
    }
  }

  async findAll() {
    try {
      const pointOfSales: PointOfSale[] = await this.prisma.pointOfSale.findMany();
      
      if (pointOfSales.length === 0) {
        return { success: true, message: 'Point-of-sale list is empty', data: [] };
      }
      
      return { success: true, data: pointOfSales };
    } catch (error) {
      console.error('Error al obtener los puntos de venta:', error);
      throw new InternalServerErrorException('Error getting point-of-sales');
    }
  }

  async findOne(id: string) {
    try {
      const pointOfSale = await this.prisma.pointOfSale.findUnique({ 
        where: { id } 
      });

      if (!pointOfSale) {
        throw new NotFoundException(`Point of Sale with ID ${id} not found`);
      }

      return { success: true, data: pointOfSale };
    } catch (error) {
      if (error instanceof NotFoundException) throw error;

      console.error('Error al buscar el punto de venta:', error);
      throw new InternalServerErrorException('Error getting Point of sale, try later');
    }
  }

  async update(id: string, updatePointOfSaleDto: UpdatePointOfSaleDto) {
    try {
      const existingPointOfSale = await this.prisma.pointOfSale.findUnique({ 
        where: { id } 
      });

      if (!existingPointOfSale) {
        throw new NotFoundException(`Point of Sale with ID ${id} not found`);
      }

      const updatedPointOfSale = await this.prisma.pointOfSale.update({
        data: updatePointOfSaleDto,
        where: { id },
      });

      return { success: true, data: updatedPointOfSale };
    } catch (error) {
      if (error instanceof NotFoundException) throw error;

      console.error('Error al actualizar el punto de venta:', error);
      throw new InternalServerErrorException('Error updating point of sale');
    }
  }

  async remove(id: string) {
    try {
      const pointOfSale = await this.prisma.pointOfSale.findUnique({ 
        where: { id } 
      });

      if (!pointOfSale) {
        throw new NotFoundException('Point of Sale not found');
      }

      // Corrección crítica: Se añadió el 'await' faltante
      await this.prisma.pointOfSale.delete({ 
        where: { id } 
      });

      return { success: true, message: 'Point of sale deleted successfully' };
    } catch (error) {
      if (error instanceof NotFoundException) throw error;

      console.error('Error al eliminar el punto de venta:', error);
      throw new InternalServerErrorException('Error deleting point of sale');
    }
  }
}