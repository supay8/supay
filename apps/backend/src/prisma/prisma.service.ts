import { Injectable } from "@nestjs/common";
import { PrismaPg } from "@prisma/adapter-pg";
import { PrismaClient } from "../../generated/prisma/client";
import { config } from "dotenv";
import { resolve } from "path";

// Forzamos la carga del .env subiendo los niveles necesarios desde dist hasta la raíz del backend o del monorepo
// Si este archivo compila en dist/src/prisma/, necesitamos subir más niveles, o mejor aún, apuntar directo a process.cwd()
config({
  path: resolve(process.cwd(), '../../.env'),
});

@Injectable()
export class PrismaService extends PrismaClient {
  constructor() {
    const adapter = new PrismaPg({
      connectionString: process.env.DATABASE_URL as string,
    });
    
    super({ adapter });
  }
}