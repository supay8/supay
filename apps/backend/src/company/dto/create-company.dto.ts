import { IsEnum, IsNotEmpty, IsOptional, IsString } from 'class-validator';
import { SiatEnvironment } from '../../../generated/prisma/enums';


export class CreateCompanyDto {
  @IsString({ message: 'El NIT debe ser una cadena de texto' })
  @IsNotEmpty({ message: 'El NIT es obligatorio' })
  nit: string;

  @IsString()
  @IsNotEmpty({ message: 'La razón social (businessName) es obligatoria' })
  businessName: string;

  @IsString()
  @IsNotEmpty({ message: 'El código de sistema es obligatorio' })
  codigoSistema: string;

  
  @IsOptional()
  @IsEnum(SiatEnvironment, { 
    message: 'El ambiente debe ser PILOTO o PRODUCCION' 
  })
  ambiente?: SiatEnvironment;
}