/*
  Warnings:

  - You are about to drop the `User` table. If the table is not empty, all the data it contains will be lost.

*/
-- CreateEnum
CREATE TYPE "OrderSide" AS ENUM ('BUY', 'SELL');

-- CreateEnum
CREATE TYPE "OrderType" AS ENUM ('MARKET', 'LIMIT', 'STOP_LIMIT', 'STOP_MARKET');

-- CreateEnum
CREATE TYPE "OrderStatus" AS ENUM ('PENDING', 'OPEN', 'PARTIAL_FILLED', 'FILLED', 'CANCELLED', 'REJECTED', 'EXPIRED');

-- CreateEnum
CREATE TYPE "TimeInForce" AS ENUM ('GTC', 'IOC', 'FOK', 'GTD');

-- CreateEnum
CREATE TYPE "TradeRole" AS ENUM ('MAKER', 'TAKER');

-- DropTable
DROP TABLE "public"."User";

-- CreateTable
CREATE TABLE "orders" (
    "id" UUID NOT NULL DEFAULT gen_random_uuid(),
    "symbol" TEXT NOT NULL,
    "side" "OrderSide" NOT NULL,
    "type" "OrderType" NOT NULL,
    "quantity" DECIMAL(20,8) NOT NULL,
    "price" DECIMAL(20,8),
    "status" "OrderStatus" NOT NULL,
    "filled_quantity" DECIMAL(20,8) NOT NULL DEFAULT 0,
    "remaining_quantity" DECIMAL(20,8) NOT NULL,
    "average_fill_price" DECIMAL(20,8),
    "time_in_force" "TimeInForce" NOT NULL DEFAULT 'GTC',
    "stop_price" DECIMAL(20,8),
    "fee_amount" DECIMAL(20,8),
    "fee_currency" TEXT,
    "user_id" UUID NOT NULL,
    "created_at" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP(3) NOT NULL,
    "cancelled_at" TIMESTAMP(3),
    "filled_at" TIMESTAMP(3),

    CONSTRAINT "orders_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "trades" (
    "id" UUID NOT NULL DEFAULT gen_random_uuid(),
    "symbol" TEXT NOT NULL,
    "price" DECIMAL(20,8) NOT NULL,
    "quantity" DECIMAL(20,8) NOT NULL,
    "maker_order_id" UUID NOT NULL,
    "taker_order_id" UUID NOT NULL,
    "maker_user_id" UUID NOT NULL,
    "taker_user_id" UUID NOT NULL,
    "maker_side" "OrderSide" NOT NULL,
    "taker_side" "OrderSide" NOT NULL,
    "maker_fee" DECIMAL(20,8) NOT NULL DEFAULT 0,
    "taker_fee" DECIMAL(20,8) NOT NULL DEFAULT 0,
    "executed_at" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT "trades_pkey" PRIMARY KEY ("id")
);

-- CreateTable
CREATE TABLE "users" (
    "id" UUID NOT NULL DEFAULT gen_random_uuid(),
    "email" TEXT NOT NULL,
    "username" TEXT,
    "created_at" TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMP(3) NOT NULL,

    CONSTRAINT "users_pkey" PRIMARY KEY ("id")
);

-- CreateIndex
CREATE INDEX "orders_user_id_idx" ON "orders"("user_id");

-- CreateIndex
CREATE INDEX "orders_symbol_status_idx" ON "orders"("symbol", "status");

-- CreateIndex
CREATE INDEX "orders_created_at_idx" ON "orders"("created_at");

-- CreateIndex
CREATE INDEX "orders_status_symbol_idx" ON "orders"("status", "symbol");

-- CreateIndex
CREATE INDEX "trades_symbol_executed_at_idx" ON "trades"("symbol", "executed_at");

-- CreateIndex
CREATE INDEX "trades_maker_order_id_idx" ON "trades"("maker_order_id");

-- CreateIndex
CREATE INDEX "trades_taker_order_id_idx" ON "trades"("taker_order_id");

-- CreateIndex
CREATE INDEX "trades_maker_user_id_idx" ON "trades"("maker_user_id");

-- CreateIndex
CREATE INDEX "trades_taker_user_id_idx" ON "trades"("taker_user_id");

-- CreateIndex
CREATE INDEX "trades_executed_at_idx" ON "trades"("executed_at");

-- CreateIndex
CREATE UNIQUE INDEX "users_email_key" ON "users"("email");

-- CreateIndex
CREATE UNIQUE INDEX "users_username_key" ON "users"("username");

-- AddForeignKey
ALTER TABLE "orders" ADD CONSTRAINT "orders_user_id_fkey" FOREIGN KEY ("user_id") REFERENCES "users"("id") ON DELETE RESTRICT ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "trades" ADD CONSTRAINT "trades_maker_order_id_fkey" FOREIGN KEY ("maker_order_id") REFERENCES "orders"("id") ON DELETE RESTRICT ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "trades" ADD CONSTRAINT "trades_taker_order_id_fkey" FOREIGN KEY ("taker_order_id") REFERENCES "orders"("id") ON DELETE RESTRICT ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "trades" ADD CONSTRAINT "trades_maker_user_id_fkey" FOREIGN KEY ("maker_user_id") REFERENCES "users"("id") ON DELETE RESTRICT ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "trades" ADD CONSTRAINT "trades_taker_user_id_fkey" FOREIGN KEY ("taker_user_id") REFERENCES "users"("id") ON DELETE RESTRICT ON UPDATE CASCADE;
