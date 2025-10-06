import express, { type Express, type NextFunction, type Request, type Response } from "express";
import type { HttpError } from "http-errors";
import { logger } from "@repo/logger";
import tradeRouter from "@/routers/trade.route";
import userRouter from "@/routers/user.route";

const app: Express = express();
app.use(express.json());

app.use("/api/v1/trade", tradeRouter);
app.use("/api/v1/user", userRouter);

app.get("/", (_, res) => {
  res.json({ message: "Hello World from Nerve trade platform's backend" });
});

// eslint-disable-next-line @typescript-eslint/no-unused-vars
app.use((err: HttpError, req: Request, res: Response, _next: NextFunction) => {
  if (err instanceof Error) {
    const statusCode = err.status || err.statusCode || 500;

    logger.error({
      message: err.message,
      name: err.name,
      stack: err.stack,
      method: req.method,
      path: req.originalUrl,
      params: req.params,
      query: req.query,
      body: { ...req.body, password: null },
    });

    // Send response to client
    res.status(statusCode).json({
      error: [
        {
          type: err.name,
          msg: err.message,
          method: req.method,
          path: req.originalUrl,
        },
      ],
    });
  }
});

export default app;
