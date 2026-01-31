const KAFKA_TOPICS = {
  ENGINE_ENVENTS: "engine-events",
  TRADES: "trades",
  PAYMENTS: "payments",
  USERS: "users",
};

const ORDER_EVENTS = {
  CREATE: "create",
  UPDATE: "update",
  DELETE: "delete",
};

const KAFKA_CONSUMER_GROUP_ID = {
  ORDER_CONSUMER_SERVICE: "order-consumer-service",
  ORDER_CONSUMER_SERVICE_1: "order-consumer-service-1",
  ORDER_CONSUMER_SERVICE_2: "order-consumer-service-2",
  ORDER_CONSUMER_SERVICE_3: "order-consumer-service-3",
};

const KAFKA_CLIENT_ID = {
  ORDER_PRODUCER_SERVICE: "order-producer-service",
  ORDER_CONSUMER_SERVICE: "order-consumer-service",
};

export { KAFKA_TOPICS, KAFKA_CONSUMER_GROUP_ID, KAFKA_CLIENT_ID, ORDER_EVENTS };
