/*
  Warnings:

  - You are about to drop the column `user_id` on the `trades` table. All the data in the column will be lost.

*/
-- DropForeignKey
ALTER TABLE "public"."trades" DROP CONSTRAINT "trades_user_id_fkey";

-- AlterTable
ALTER TABLE "trades" DROP COLUMN "user_id",
ADD COLUMN     "buyer_id" UUID,
ADD COLUMN     "seller_id" UUID;

-- AddForeignKey
ALTER TABLE "trades" ADD CONSTRAINT "trades_buyer_id_fkey" FOREIGN KEY ("buyer_id") REFERENCES "users"("id") ON DELETE SET NULL ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "trades" ADD CONSTRAINT "trades_seller_id_fkey" FOREIGN KEY ("seller_id") REFERENCES "users"("id") ON DELETE SET NULL ON UPDATE CASCADE;
