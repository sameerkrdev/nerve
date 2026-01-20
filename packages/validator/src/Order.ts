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

export const ModifyOrderValidator = z.object({
  params: z.object({
    id: z.uuid(),
  }),

  body: z
    .object({
      symbol: z.string().trim(),

      newPrice: z.number().int().positive().optional(),
      newQuantity: z.number().int().positive().optional(),
    })
    .refine(
      (data) => {
        return data.newPrice !== undefined || data.newQuantity !== undefined;
      },
      {
        message: "At least one of newPrice or newQuantity must be provided",
        path: ["body"],
      },
    ),
});
export type ModifyOrder = z.infer<typeof ModifyOrderValidator>;
