import { logger } from "@repo/logger";
import express, { type Express, type Request, type Response, type NextFunction } from "express";
import type { HttpError } from "http-errors";

const app: Express = express();

app.use(express.json());

app.get("/", (_, res) => {
  res.json({ message: "Hello World from order service" });
});

// eslint-disable-next-line @typescript-eslint/no-unused-vars
app.use((err: HttpError, _req: Request, res: Response, _next: NextFunction) => {
  if (err instanceof Error) {
    logger.error(err.message);
    const statusCode = err.status || err.statusCode || 500;

    res.status(statusCode).json({
      error: [
        {
          type: err.name,
          msg: err.message,
          path: "",
          location: "",
        },
      ],
    });
  }
});

export default app;
