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
        maker_order: true,
        taker_order: true,
        maker_user: true,
        taker_user: true,
      },
    });
  }

  /**
   * Get all trades for a specific symbol
   */
  async findBySymbol(symbol: string, limit = 100): Promise<Trade[]> {
    return this.client.trade.findMany({
      where: { symbol },
      orderBy: { executed_at: "desc" },
      take: limit,
    });
  }

  /**
   * Get all trades of a user (either as maker or taker)
   */
  async findByUser(userId: string, limit = 100): Promise<Trade[]> {
    return this.client.trade.findMany({
      where: {
        OR: [{ maker_user_id: userId }, { taker_user_id: userId }],
      },
      orderBy: { executed_at: "desc" },
      take: limit,
    });
  }

  /**
   * Fetch trades between two users (e.g., counterparty analysis)
   */
  async findBetweenUsers(userA: string, userB: string): Promise<Trade[]> {
    return this.client.trade.findMany({
      where: {
        OR: [
          { maker_user_id: userA, taker_user_id: userB },
          { maker_user_id: userB, taker_user_id: userA },
        ],
      },
      orderBy: { executed_at: "desc" },
    });
  }

  /**
   * Delete trade (mostly for testing)
   */
  async delete(id: string): Promise<void> {
    await this.client.trade.delete({ where: { id } });
  }
}
