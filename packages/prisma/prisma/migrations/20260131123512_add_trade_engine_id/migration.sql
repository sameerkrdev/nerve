/*
  Warnings:

  - Added the required column `engine_id` to the `trades` table without a default value. This is not possible if the table is not empty.

*/
-- AlterTable
ALTER TABLE "trades" ADD COLUMN     "engine_id" TEXT NOT NULL;
