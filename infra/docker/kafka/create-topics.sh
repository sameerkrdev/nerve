#!/bin/bash

TOPICS=("orders" "trades" "payments" "users")
BOOTSTRAP_SERVERS="kafka-1:9092"
PARTITIONS=12
REPLICATION_FACTOR=3
MIN_INSYNC_REPLICAS=2

echo "Waiting for Kafka brokers to be ready..."
until kafka-broker-api-versions --bootstrap-server $BOOTSTRAP_SERVERS &> /dev/null
do
  sleep 2
  echo -n "."
done
echo -e "\nKafka brokers are up!"

for TOPIC in "${TOPICS[@]}"; do
  echo "Creating topic: $TOPIC"
  kafka-topics --create \
    --topic "$TOPIC" \
    --bootstrap-server $BOOTSTRAP_SERVERS \
    --partitions $PARTITIONS \
    --replication-factor $REPLICATION_FACTOR \
    --config min.insync.replicas=$MIN_INSYNC_REPLICAS \
    --config retention.ms=604800000 \
    --config segment.ms=86400000 \
    --if-not-exists
  
  if [ $? -eq 0 ]; then
    echo "✅ Topic $TOPIC created/exists"
  else
    echo "❌ Failed to create topic $TOPIC"
  fi
done

echo ""
echo "📋 Listing all topics:"
kafka-topics --list --bootstrap-server $BOOTSTRAP_SERVERS

echo ""
echo "📊 Topic details:"
for TOPIC in "${TOPICS[@]}"; do
  echo "--- $TOPIC ---"
  kafka-topics --describe --topic "$TOPIC" --bootstrap-server $BOOTSTRAP_SERVERS
  echo ""
done

echo "✅ All topics configured successfully!"