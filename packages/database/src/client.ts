import { createClient, type ClickHouseClient } from "@clickhouse/client";

interface IClickHouseConfig {
  url: string;
  database: string;
  password: string;
  username: string;
}

class ClickHouseManager {
  private static instance: ClickHouseManager;
  private clickhouseClient: ClickHouseClient | null = null;

  private constructor() {}

  public static getInstance(): ClickHouseManager {
    if (!ClickHouseManager.instance) {
      ClickHouseManager.instance = new ClickHouseManager();
    }
    return ClickHouseManager.instance;
  }

  public async initialize(config: IClickHouseConfig): Promise<ClickHouseClient> {
    if (this.clickhouseClient) return this.clickhouseClient;

    const { url, database, username, password } = config;

    this.clickhouseClient = createClient({
      url,
      username,
      password,
      database,
      max_open_connections: 10,
      request_timeout: 30000,
      compression: { response: true, request: false },
      keep_alive: { enabled: true, idle_socket_ttl: 2500 },
    });

    await this.clickhouseClient.ping();
    return this.clickhouseClient;
  }

  public getClickhouseClient(): ClickHouseClient {
    if (!this.clickhouseClient) {
      throw new Error("ClickHouse Client is not initialized. Call initialize() first.");
    }
    return this.clickhouseClient;
  }

  public async close(): Promise<void> {
    if (this.clickhouseClient) {
      await this.clickhouseClient.close();
      this.clickhouseClient = null;
    }
  }
}

export const clickHouseManager = ClickHouseManager.getInstance();
export type { IClickHouseConfig };
