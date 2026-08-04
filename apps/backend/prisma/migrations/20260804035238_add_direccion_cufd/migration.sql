/*
  Warnings:

  - Added the required column `direccion` to the `Cufd` table without a default value. This is not possible if the table is not empty.

*/
-- AlterTable
ALTER TABLE "Cufd" ADD COLUMN     "direccion" TEXT NOT NULL;
