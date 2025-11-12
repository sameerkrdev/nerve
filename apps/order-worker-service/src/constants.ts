import { Side, Status, Type } from "@repo/proto-defs/ts/order_service";

const OrderStatusEnumStringMap: Record<number, string> = {
  [Status.PENDING]: "PENDING",
  [Status.OPEN]: "OPEN",
  [Status.PARTIAL_FILLED]: "PARTIAL_FILLED",
  [Status.FILLED]: "FILLED",
  [Status.CANCELLED]: "CANCELLED",
  [Status.REJECTED]: "REJECTED",
  [Status.EXPIRED]: "EXPIRED",
};

const OrderTypeEnumStringMap: Record<number, string> = {
  [Type.LIMIT]: "LIMIT",
  [Type.MARKET]: "MARKET",
  [Type.STOP_LIMIT]: "STOP_LIMIT",
  [Type.STOP_MARKET]: "STOP_MARKET",
};

const OrderSideEnumStringMap: Record<number, string> = {
  [Side.BUY]: "BUY",
  [Side.SELL]: "SELL",
};

export { OrderStatusEnumStringMap, OrderTypeEnumStringMap, OrderSideEnumStringMap };
