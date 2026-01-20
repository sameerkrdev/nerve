# Order Service

## Current Architecture

This service currently combines two responsibilities in a single process:

1. **Producer (gRPC Server)**:
   - Handles gRPC requests from API Gateway
   - Produces order events to Kafka `orders` topic
   - Client ID: `order-producer-service`

2. **Consumer (Kafka Consumer)**:
   - Consumes order events from Kafka `orders` topic
   - Persists orders to database via `OrderRepository`
   - Client ID: `order-service`, Group ID: `order-service`

## Issues with Current Setup

1. **Tight Coupling**: If the consumer fails (e.g., database connection issues, processing errors), the entire service process may crash, taking down the gRPC server and preventing new orders from being accepted.

2. **Scaling Limitations**: Cannot scale consumers independently. Deploying multiple instances to increase consumer throughput also duplicates:
   - gRPC servers (unnecessary resource usage)
   - Kafka producers (redundant connections)
   - Each instance competes for the same gRPC port

3. **Resource Efficiency**: Consumer and producer share the same process resources, making it difficult to optimize each independently.

## Proposed Solution

Split into two separate services:

- **`order-service`** (or `order-grpc-service`):
  - Only handles gRPC requests
  - Only produces to Kafka
  - Can be scaled based on gRPC request load

- **`order-persistence-service`** (or `order-consumer-service`):
  - Only consumes from Kafka
  - Only persists to database
  - Can be scaled independently based on message processing load
  - Can be deployed with multiple instances for parallel processing

## TODO

- [ ] Split consumer logic into a separate service (`order-persistence-service`)
- [ ] Keep only gRPC server and Kafka producer in `order-service`
- [ ] Update deployment configurations for independent scaling
