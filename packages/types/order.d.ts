import { Side, Type } from "@repo/proto-defs/ts/order_service";

export type OrderSideKeys = keyof typeof Side;
export type OrderTypeKeys = keyof typeof Type;
