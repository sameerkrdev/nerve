import { Side, OrderType as Type } from "@repo/proto-defs/ts/common/order_types";

export type OrderSideKeys = keyof typeof Side;
export type OrderTypeKeys = keyof typeof Type;
