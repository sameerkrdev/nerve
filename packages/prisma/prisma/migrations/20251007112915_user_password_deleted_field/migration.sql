/*
  Warnings:

  - Added the required column `deleted` to the `users` table without a default value. This is not possible if the table is not empty.
  - Added the required column `password` to the `users` table without a default value. This is not possible if the table is not empty.

*/
-- AlterTable
ALTER TABLE "users" ADD COLUMN     "deleted" BOOLEAN NOT NULL,
ADD COLUMN     "password" TEXT NOT NULL;
