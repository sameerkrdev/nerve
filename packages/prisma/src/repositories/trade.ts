import { prisma, type PrismaClientType } from "../client";
import type { Prisma, Trade } from "../../generated/prisma";

export class TradeRepository {
  constructor(private client: PrismaClientType = prisma) {}

  /**
   * Create a trade record
   */
  async create(data: Prisma.TradeCreateInput): Promise<Trade> {
    return this.client.trade.create({ data });
  }

  /**
   * Get trade by ID
   */
  async findById(id: string): Promise<Trade | null> {
    return this.client.trade.findUnique({
      where: { id },
      include: {
        buy_order: true,
        sell_order: true,
        seller: true,
        buyer: true,
      },
    });
  }

  /**
   * Get all trades for a specific symbol
   */
  async findBySymbol(symbol: string, limit = 100): Promise<Trade[]> {
    return this.client.trade.findMany({
      where: { symbol },
      orderBy: { created_at: "desc" },
      take: limit,
    });
  }

  /**
   * Get all trades of a user (either as maker or taker)
   */
  async findByUser(userId: string, limit = 100): Promise<Trade[]> {
    return this.client.trade.findMany({
      where: {
        OR: [{ seller_id: userId }, { buyer_id: userId }],
      },
      orderBy: { created_at: "desc" },
      take: limit,
    });
  }

  /**
   * Delete trade (mostly for testing)
   */
  async delete(id: string): Promise<void> {
    await this.client.trade.delete({ where: { id } });
  }
}
