/*
  Warnings:

  - You are about to drop the column `average_fill_price` on the `orders` table. All the data in the column will be lost.
  - You are about to drop the column `fee_amount` on the `orders` table. All the data in the column will be lost.
  - You are about to drop the column `fee_currency` on the `orders` table. All the data in the column will be lost.
  - You are about to drop the column `stop_price` on the `orders` table. All the data in the column will be lost.
  - You are about to drop the column `time_in_force` on the `orders` table. All the data in the column will be lost.
  - You are about to alter the column `quantity` on the `orders` table. The data in that column could be lost. The data in that column will be cast from `Decimal(20,8)` to `Integer`.
  - You are about to alter the column `price` on the `orders` table. The data in that column could be lost. The data in that column will be cast from `Decimal(20,8)` to `Integer`.
  - You are about to alter the column `filled_quantity` on the `orders` table. The data in that column could be lost. The data in that column will be cast from `Decimal(20,8)` to `Integer`.
  - You are about to alter the column `remaining_quantity` on the `orders` table. The data in that column could be lost. The data in that column will be cast from `Decimal(20,8)` to `Integer`.
  - You are about to drop the column `executed_at` on the `trades` table. All the data in the column will be lost.
  - You are about to drop the column `maker_fee` on the `trades` table. All the data in the column will be lost.
  - You are about to drop the column `maker_order_id` on the `trades` table. All the data in the column will be lost.
  - You are about to drop the column `maker_side` on the `trades` table. All the data in the column will be lost.
  - You are about to drop the column `maker_user_id` on the `trades` table. All the data in the column will be lost.
  - You are about to drop the column `taker_fee` on the `trades` table. All the data in the column will be lost.
  - You are about to drop the column `taker_order_id` on the `trades` table. All the data in the column will be lost.
  - You are about to drop the column `taker_side` on the `trades` table. All the data in the column will be lost.
  - You are about to drop the column `taker_user_id` on the `trades` table. All the data in the column will be lost.
  - You are about to alter the column `price` on the `trades` table. The data in that column could be lost. The data in that column will be cast from `Decimal(20,8)` to `Integer`.
  - You are about to alter the column `quantity` on the `trades` table. The data in that column could be lost. The data in that column will be cast from `Decimal(20,8)` to `Integer`.
  - Added the required column `average_price` to the `orders` table without a default value. This is not possible if the table is not empty.
  - Added the required column `canelled_quantity` to the `orders` table without a default value. This is not possible if the table is not empty.
  - Added the required column `client_timeline` to the `orders` table without a default value. This is not possible if the table is not empty.
  - Added the required column `engine_timeline` to the `orders` table without a default value. This is not possible if the table is not empty.
  - Added the required column `executedValue` to the `orders` table without a default value. This is not possible if the table is not empty.
  - Added the required column `gateway_timeline` to the `orders` table without a default value. This is not possible if the table is not empty.
  - Made the column `price` on table `orders` required. This step will fail if there are existing NULL values in that column.
  - Added the required column `is_buyer_maker` to the `trades` table without a default value. This is not possible if the table is not empty.
  - Added the required column `trade_sequence` to the `trades` table without a default value. This is not possible if the table is not empty.

*/
-- DropForeignKey
ALTER TABLE "public"."trades" DROP CONSTRAINT "trades_maker_order_id_fkey";

-- DropForeignKey
ALTER TABLE "public"."trades" DROP CONSTRAINT "trades_maker_user_id_fkey";

-- DropForeignKey
ALTER TABLE "public"."trades" DROP CONSTRAINT "trades_taker_order_id_fkey";

-- DropForeignKey
ALTER TABLE "public"."trades" DROP CONSTRAINT "trades_taker_user_id_fkey";

-- DropIndex
DROP INDEX "public"."orders_status_symbol_idx";

-- DropIndex
DROP INDEX "public"."trades_executed_at_idx";

-- DropIndex
DROP INDEX "public"."trades_maker_order_id_idx";

-- DropIndex
DROP INDEX "public"."trades_maker_user_id_idx";

-- DropIndex
DROP INDEX "public"."trades_symbol_executed_at_idx";

-- DropIndex
DROP INDEX "public"."trades_taker_order_id_idx";

-- DropIndex
DROP INDEX "public"."trades_taker_user_id_idx";

-- AlterTable
ALTER TABLE "orders" DROP COLUMN "average_fill_price",
DROP COLUMN "fee_amount",
DROP COLUMN "fee_currency",
DROP COLUMN "stop_price",
DROP COLUMN "time_in_force",
ADD COLUMN     "average_price" INTEGER NOT NULL,
ADD COLUMN     "canelled_quantity" INTEGER NOT NULL,
ADD COLUMN     "client_timeline" TIMESTAMP(3) NOT NULL,
ADD COLUMN     "currency" TEXT,
ADD COLUMN     "engine_timeline" TIMESTAMP(3) NOT NULL,
ADD COLUMN     "executedValue" INTEGER NOT NULL,
ADD COLUMN     "gateway_timeline" TIMESTAMP(3) NOT NULL,
ADD COLUMN     "status_message" TEXT,
ALTER COLUMN "quantity" SET DATA TYPE INTEGER,
ALTER COLUMN "price" SET NOT NULL,
ALTER COLUMN "price" SET DATA TYPE INTEGER,
ALTER COLUMN "filled_quantity" DROP DEFAULT,
ALTER COLUMN "filled_quantity" SET DATA TYPE INTEGER,
ALTER COLUMN "remaining_quantity" DROP DEFAULT,
ALTER COLUMN "remaining_quantity" SET DATA TYPE INTEGER;

-- AlterTable
ALTER TABLE "trades" DROP COLUMN "executed_at",
DROP COLUMN "maker_fee",
DROP COLUMN "maker_order_id",
DROP COLUMN "maker_side",
DROP COLUMN "maker_user_id",
DROP COLUMN "taker_fee",
DROP COLUMN "taker_order_id",
DROP COLUMN "taker_side",
DROP COLUMN "taker_user_id",
ADD COLUMN     "buy_order_id" UUID,
ADD COLUMN     "is_buyer_maker" BOOLEAN NOT NULL,
ADD COLUMN     "sell_order_id" UUID,
ADD COLUMN     "trade_sequence" INTEGER NOT NULL,
ADD COLUMN     "user_id" UUID,
ALTER COLUMN "price" SET DATA TYPE INTEGER,
ALTER COLUMN "quantity" SET DATA TYPE INTEGER;

-- AddForeignKey
ALTER TABLE "trades" ADD CONSTRAINT "trades_sell_order_id_fkey" FOREIGN KEY ("sell_order_id") REFERENCES "orders"("id") ON DELETE SET NULL ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "trades" ADD CONSTRAINT "trades_buy_order_id_fkey" FOREIGN KEY ("buy_order_id") REFERENCES "orders"("id") ON DELETE SET NULL ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "trades" ADD CONSTRAINT "trades_user_id_fkey" FOREIGN KEY ("user_id") REFERENCES "users"("id") ON DELETE SET NULL ON UPDATE CASCADE;
