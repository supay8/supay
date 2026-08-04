-- CreateEnum
CREATE TYPE "SiatEnvironment" AS ENUM ('PILOTO', 'PRODUCCION');

-- CreateEnum
CREATE TYPE "InvoiceStatus" AS ENUM ('PENDING', 'SENT', 'ACCEPTED', 'REJECTED', 'OFFLINE', 'CANCELLED');

-- CreateEnum
CREATE TYPE "DocumentType" AS ENUM ('CI', 'CEX', 'PAS', 'NIT', 'OD');

-- CreateEnum
CREATE TYPE "EmissionType" AS ENUM ('EN_LINEA', 'OFFLINE', 'CONTINGENCIA');

-- CreateEnum
CREATE TYPE "ContingencyReason" AS ENUM ('FALTA_ENERGIA_ELECTRICA', 'FALLA_CONEXION_INTERNET', 'FALLA_SERVIDOR_SIN', 'FALLA_SISTEMA_FACTURACION', 'OTRO');

-- CreateTable
CREATE TABLE "Company" (
    "id" TEXT NOT NULL,
    "nit" TEXT NOT NULL,
    "businessName" TEXT NOT NULL,
    "codigoSistema" TEXT NOT NULL,
    "ambiente" "SiatEnvironment" NOT NULL DEFAULT 'PILOTO',
    "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updatedAt" TIMESTAMP(3) NOT NULL,

    CONSTRAINT "Company_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "PointOfSale" (
    "id" TEXT NOT NULL,
    "companyId" TEXT NOT NULL,
    "codigoSucursal" INTEGER NOT NULL DEFAULT 0,
    "codigoPuntoVenta" INTEGER NOT NULL,
    "description" TEXT NOT NULL,
    "cuis" TEXT,
    "cuisCreatedAt" TIMESTAMP(3),
    "isActive" BOOLEAN NOT NULL DEFAULT true,
    "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT "PointOfSale_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "Cufd" (
    "id" TEXT NOT NULL,
    "pointOfSaleId" TEXT NOT NULL,
    "cufd" TEXT NOT NULL,
    "codigoControl" TEXT NOT NULL,
    "validFrom" TIMESTAMP(3) NOT NULL,
    "validTo" TIMESTAMP(3) NOT NULL,

    CONSTRAINT "Cufd_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "ContingencyEvent" (
    "id" TEXT NOT NULL,
    "pointOfSaleId" TEXT NOT NULL,
    "reason" "ContingencyReason" NOT NULL,
    "description" TEXT,
    "startDate" TIMESTAMP(3) NOT NULL,
    "endDate" TIMESTAMP(3),
    "siatEventCode" TEXT,
    "isSynced" BOOLEAN NOT NULL DEFAULT false,
    "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT "ContingencyEvent_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "Customer" (
    "id" TEXT NOT NULL,
    "companyId" TEXT NOT NULL,
    "documentType" "DocumentType" NOT NULL,
    "documentNumber" TEXT NOT NULL,
    "complement" TEXT,
    "name" TEXT NOT NULL,

    CONSTRAINT "Customer_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "Invoice" (
    "id" TEXT NOT NULL,
    "companyId" TEXT NOT NULL,
    "customerId" TEXT NOT NULL,
    "pointOfSaleId" TEXT NOT NULL,
    "cufdId" TEXT NOT NULL,
    "contingencyEventId" TEXT,
    "invoiceNumber" INTEGER NOT NULL,
    "cuf" TEXT,
    "emissionType" "EmissionType" NOT NULL DEFAULT 'EN_LINEA',
    "codigoMetodoPago" INTEGER NOT NULL DEFAULT 1,
    "codigoMoneda" INTEGER NOT NULL DEFAULT 1,
    "tipoCambio" DECIMAL(18,5) NOT NULL DEFAULT 1,
    "issueDate" TIMESTAMP(3) NOT NULL,
    "subtotal" DECIMAL(18,2) NOT NULL,
    "discount" DECIMAL(18,2) NOT NULL DEFAULT 0,
    "total" DECIMAL(18,2) NOT NULL,
    "xml" TEXT,
    "xmlHash" TEXT,
    "siatReceptionCode" TEXT,
    "status" "InvoiceStatus" NOT NULL DEFAULT 'PENDING',
    "motivoAnulacion" INTEGER,
    "fechaAnulacion" TIMESTAMP(3),
    "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT "Invoice_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "InvoiceItem" (
    "id" TEXT NOT NULL,
    "invoiceId" TEXT NOT NULL,
    "code" TEXT NOT NULL,
    "description" TEXT NOT NULL,
    "codigoActividad" TEXT,
    "codigoProductoSin" TEXT,
    "unitCode" INTEGER,
    "quantity" DECIMAL(18,3) NOT NULL,
    "unitPrice" DECIMAL(18,2) NOT NULL,
    "discount" DECIMAL(18,2) NOT NULL DEFAULT 0,
    "subtotal" DECIMAL(18,2) NOT NULL,

    CONSTRAINT "InvoiceItem_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "InvoiceEvent" (
    "id" TEXT NOT NULL,
    "invoiceId" TEXT NOT NULL,
    "type" TEXT NOT NULL,
    "message" TEXT NOT NULL,
    "payload" JSONB,
    "createdAt" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT "InvoiceEvent_pkey" PRIMARY KEY ("id")
);

-- CreateIndex
CREATE UNIQUE INDEX "Company_nit_key" ON "Company"("nit");

-- CreateIndex
CREATE UNIQUE INDEX "PointOfSale_companyId_codigoSucursal_codigoPuntoVenta_key" ON "PointOfSale"("companyId", "codigoSucursal", "codigoPuntoVenta");

-- CreateIndex
CREATE INDEX "Cufd_pointOfSaleId_validFrom_idx" ON "Cufd"("pointOfSaleId", "validFrom");

-- CreateIndex
CREATE INDEX "ContingencyEvent_pointOfSaleId_startDate_idx" ON "ContingencyEvent"("pointOfSaleId", "startDate");

-- CreateIndex
CREATE UNIQUE INDEX "Customer_companyId_documentType_documentNumber_key" ON "Customer"("companyId", "documentType", "documentNumber");

-- CreateIndex
CREATE UNIQUE INDEX "Invoice_cuf_key" ON "Invoice"("cuf");

-- CreateIndex
CREATE INDEX "Invoice_status_idx" ON "Invoice"("status");

-- CreateIndex
CREATE INDEX "Invoice_issueDate_idx" ON "Invoice"("issueDate");

-- CreateIndex
CREATE UNIQUE INDEX "Invoice_pointOfSaleId_invoiceNumber_key" ON "Invoice"("pointOfSaleId", "invoiceNumber");

-- AddForeignKey
ALTER TABLE "PointOfSale" ADD CONSTRAINT "PointOfSale_companyId_fkey" FOREIGN KEY ("companyId") REFERENCES "Company"("id") ON DELETE RESTRICT ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "Cufd" ADD CONSTRAINT "Cufd_pointOfSaleId_fkey" FOREIGN KEY ("pointOfSaleId") REFERENCES "PointOfSale"("id") ON DELETE RESTRICT ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "ContingencyEvent" ADD CONSTRAINT "ContingencyEvent_pointOfSaleId_fkey" FOREIGN KEY ("pointOfSaleId") REFERENCES "PointOfSale"("id") ON DELETE RESTRICT ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "Customer" ADD CONSTRAINT "Customer_companyId_fkey" FOREIGN KEY ("companyId") REFERENCES "Company"("id") ON DELETE RESTRICT ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "Invoice" ADD CONSTRAINT "Invoice_companyId_fkey" FOREIGN KEY ("companyId") REFERENCES "Company"("id") ON DELETE RESTRICT ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "Invoice" ADD CONSTRAINT "Invoice_customerId_fkey" FOREIGN KEY ("customerId") REFERENCES "Customer"("id") ON DELETE RESTRICT ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "Invoice" ADD CONSTRAINT "Invoice_pointOfSaleId_fkey" FOREIGN KEY ("pointOfSaleId") REFERENCES "PointOfSale"("id") ON DELETE RESTRICT ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "Invoice" ADD CONSTRAINT "Invoice_cufdId_fkey" FOREIGN KEY ("cufdId") REFERENCES "Cufd"("id") ON DELETE RESTRICT ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "Invoice" ADD CONSTRAINT "Invoice_contingencyEventId_fkey" FOREIGN KEY ("contingencyEventId") REFERENCES "ContingencyEvent"("id") ON DELETE SET NULL ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "InvoiceItem" ADD CONSTRAINT "InvoiceItem_invoiceId_fkey" FOREIGN KEY ("invoiceId") REFERENCES "Invoice"("id") ON DELETE RESTRICT ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "InvoiceEvent" ADD CONSTRAINT "InvoiceEvent_invoiceId_fkey" FOREIGN KEY ("invoiceId") REFERENCES "Invoice"("id") ON DELETE RESTRICT ON UPDATE CASCADE;
