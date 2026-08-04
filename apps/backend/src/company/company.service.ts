import { 
  Injectable, 
  InternalServerErrorException, 
  NotFoundException 
} from '@nestjs/common';
import { CreateCompanyDto } from './dto/create-company.dto';
import { UpdateCompanyDto } from './dto/update-company.dto';
import { Company } from './entities/company.entity';
import { PrismaService } from '../prisma/prisma.service';

@Injectable()
export class CompanyService {
  constructor(private readonly prisma: PrismaService) {}

  async create(createCompanyDto: CreateCompanyDto) {
    try {
      const company = await this.prisma.company.create({ 
        data: createCompanyDto 
      });
      return { success: true, data: company };
    } catch (error) {
      console.error('Error al crear la compañía:', error);
      throw new InternalServerErrorException('Error creating company, try later');
    }
  }

  async findAll() {
    try {
      const companies: Company[] = await this.prisma.company.findMany();
      
      if (companies.length === 0) {
        return { success: true, message: 'Companies list is empty', data: [] };
      }
      
      return { success: true, data: companies };
    } catch (error) {
      console.error('Error al obtener las compañías:', error);
      throw new InternalServerErrorException('Error getting companies');
    }
  }

  async findOne(id: string) {
    try {
      const company = await this.prisma.company.findUnique({ 
        where: { id } 
      });

      if (!company) {
        throw new NotFoundException(`Company with ID ${id} not found`);
      }

      return { success: true, data: company };
    } catch (error) {
      if (error instanceof NotFoundException) throw error;
      
      console.error('Error al buscar la compañía:', error);
      throw new InternalServerErrorException('Error finding company');
    }
  }

  async update(id: string, updateCompanyDto: UpdateCompanyDto) {
    try {
      // Opcional: Verificar si existe antes de actualizar para dar un mensaje más limpio
      const existingCompany = await this.prisma.company.findUnique({ where: { id } });
      if (!existingCompany) {
        throw new NotFoundException(`Company with ID ${id} not found`);
      }

      const updatedCompany = await this.prisma.company.update({
        data: updateCompanyDto,
        where: { id },
      });

      return { success: true, data: updatedCompany };
    } catch (error) {
      if (error instanceof NotFoundException) throw error;

      console.error('Error al actualizar la compañía:', error);
      throw new InternalServerErrorException('Error updating company, try later');
    }
  }

  async remove(id: string) {
    try {
      const existingCompany = await this.prisma.company.findUnique({ where: { id } });
      if (!existingCompany) {
        throw new NotFoundException(`Company with ID ${id} not found`);
      }

      await this.prisma.company.delete({ 
        where: { id } 
      });

      return { success: true, message: 'Company deleted successfully' };
    } catch (error) {
      if (error instanceof NotFoundException) throw error;

      console.error('Error al eliminar la compañía:', error);
      throw new InternalServerErrorException('Error deleting company, try later');
    }
  }
}