import winston from "winston";
import DailyRotateFile from "winston-daily-rotate-file";

export interface LoggerConfig {
  service?: string;
  level?: string;
  enableConsole?: boolean;
  enableFile?: boolean;
  logDir?: string;
  maxSize?: string;
  maxFiles?: string;
}

const createLogger = (config: LoggerConfig = {}) => {
  const {
    service = "app",
    level = process.env.LOG_LEVEL || "info",
    enableConsole = true,
    enableFile = true,
    logDir = "logs",
    maxSize = "20m",
    maxFiles = "14d",
  } = config;

  const formats = [
    winston.format.timestamp({
      format: "YYYY-MM-DD HH:mm:ss",
    }),
    winston.format.errors({ stack: true }),
    winston.format.json(),
  ];

  // Add colorize for console output
  const consoleFormats = [
    winston.format.colorize(),
    winston.format.printf(({ timestamp, level, message, service, ...meta }) => {
      const metaString = Object.keys(meta).length ? JSON.stringify(meta, null, 2) : "";
      return `${timestamp} [${service}] ${level}: ${message} ${metaString}`;
    }),
  ];

  const transports: winston.transport[] = [];

  // Console transport
  if (enableConsole) {
    transports.push(
      new winston.transports.Console({
        format: winston.format.combine(...consoleFormats),
      }),
    );
  }

  // File transports
  if (enableFile) {
    // Error logs
    transports.push(
      new DailyRotateFile({
        filename: `${logDir}/error-%DATE%.log`,
        datePattern: "YYYY-MM-DD",
        level: "error",
        maxSize,
        maxFiles,
        format: winston.format.combine(...formats),
      }),
    );

    // Combined logs
    transports.push(
      new DailyRotateFile({
        filename: `${logDir}/combined-%DATE%.log`,
        datePattern: "YYYY-MM-DD",
        maxSize,
        maxFiles,
        format: winston.format.combine(...formats),
      }),
    );
  }

  return winston.createLogger({
    level,
    defaultMeta: { service },
    format: winston.format.combine(...formats),
    transports,
    // Handle uncaught exceptions and rejections
    exceptionHandlers: enableFile
      ? [
          new DailyRotateFile({
            filename: `${logDir}/exceptions-%DATE%.log`,
            datePattern: "YYYY-MM-DD",
            maxSize,
            maxFiles,
          }),
        ]
      : [],
    rejectionHandlers: enableFile
      ? [
          new DailyRotateFile({
            filename: `${logDir}/rejections-%DATE%.log`,
            datePattern: "YYYY-MM-DD",
            maxSize,
            maxFiles,
          }),
        ]
      : [],
  });
};

// Default logger instance
export const logger = createLogger();

// Export the factory function
export { createLogger };

// Export winston types for convenience
export type Logger = winston.Logger;
