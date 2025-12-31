import * as z from "zod";

export const PlaceOrderValidator = z
  .object({
    body: z.object({
      symbol: z.string(),
      price: z.number().int().positive().optional(),
      quantity: z.number().int().positive(),
      side: z.enum(["BUY", "SELL"]),
      type: z.enum(["MARKET", "LIMIT", "STOP", "STOP_LIMIT"]),
    }),
  })
  .refine(
    (data) => {
      const { type, price } = data.body;
      if (type !== "MARKET" && price === undefined) {
        return false;
      }
      return true;
    },
    {
      message: "Price is required for LIMIT, STOP, and STOP_LIMIT orders",
      path: ["body", "price"],
    },
  );

export type PlaceOrder = z.infer<typeof PlaceOrderValidator>;

export const CancelOrderValidator = z.object({
  params: z.object({
    id: z.uuid(),
  }),
  body: z.object({
    symbol: z.string(),
  }),
});
export type CancelOrder = z.infer<typeof CancelOrderValidator>;
