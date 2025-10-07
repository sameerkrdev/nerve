import type { Prisma, Order, OrderStatus, OrderSide } from "../../generated/prisma";
import { prisma, type PrismaClientType } from "../client";

export class OrderRepository {
  constructor(private client: PrismaClientType = prisma) {}

  /**
   * Create a new order
   */
  async create(data: Prisma.OrderCreateInput): Promise<Order> {
    return this.client.order.create({ data });
  }

  /**
   * Get order by ID
   */
  async findById(id: string): Promise<Order | null> {
    return this.client.order.findUnique({
      where: { id },
      include: {
        user: true,
        maker_trades: true,
        taker_trades: true,
      },
    });
  }

  /**
   * Get all orders for a user (optionally filtered by status or symbol)
   */
  async findByUser(
    userId: string,
    filters?: { status?: OrderStatus; symbol?: string },
  ): Promise<Order[]> {
    return this.client.order.findMany({
      where: {
        user_id: userId,
        ...(filters?.status && { status: filters.status }),
        ...(filters?.symbol && { symbol: filters.symbol }),
      },
      orderBy: { created_at: "desc" },
    });
  }

  /**
   * Update an order’s status or fields
   */
  async update(id: string, data: Prisma.OrderUpdateInput): Promise<Order> {
    return this.client.order.update({
      where: { id },
      data,
    });
  }

  /**
   * Cancel an order
   */
  async cancel(id: string): Promise<Order> {
    return this.client.order.update({
      where: { id },
      data: {
        status: "CANCELLED",
        cancelled_at: new Date(),
      },
    });
  }

  /**
   * Fetch open orders (useful for matching engine)
   */
  async findOpenOrders(symbol: string, side?: OrderSide): Promise<Order[]> {
    return this.client.order.findMany({
      where: {
        symbol,
        status: "OPEN",
        ...(side && { side }),
      },
      orderBy: {
        created_at: "asc",
      },
    });
  }

  /**
   * Delete (hard remove) an order
   */
  async delete(id: string): Promise<void> {
    await this.client.order.delete({ where: { id } });
  }
}
