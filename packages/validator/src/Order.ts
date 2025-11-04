import * as z from "zod";

export const PlaceOrderValidator = z.object({
  body: z.object({
    symbol: z.string(),
    price: z.number().int().nonnegative(),
    quantity: z.number().int().nonnegative(),
    side: z.enum(["BUY", "SELL"]),
    type: z.enum(["MARKET", "LIMIT", "STOP", "STOP_LIMIT"]),
  }),
});

export type PlaceOrder = z.infer<typeof PlaceOrderValidator>;
