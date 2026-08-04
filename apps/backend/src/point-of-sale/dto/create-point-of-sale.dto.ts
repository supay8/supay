import { IsString, IsInt, IsBoolean, IsOptional, IsNotEmpty, Min } from 'class-validator';

export class CreatePointOfSaleDto {
  @IsString()
  @IsNotEmpty()
  companyId: string;

  @IsInt()
  @Min(0)
  @IsOptional()
  codigoSucursal?: number;

  @IsInt()
  @Min(0)
  @IsOptional()
  codigoPuntoVenta?: number;

  @IsString()
  @IsNotEmpty()
  description: string;

  @IsString()
  @IsOptional()
  cuis?: string;

  @IsBoolean()
  @IsOptional()
  isActive?: boolean;
}