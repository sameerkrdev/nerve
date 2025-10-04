import express, { type Express, type NextFunction, type Request, type Response } from "express";
import type { HttpError } from "http-errors";
import { logger } from "@repo/logger";
import tradeRouter from "@/routers/trade.route";

const app: Express = express();
app.use(express.json());

app.use("/api/v1/trade", tradeRouter);

app.get("/", (_, res) => {
  res.json({ message: "Hello World from Nerve trade platform's backend" });
});

// eslint-disable-next-line @typescript-eslint/no-unused-vars
app.use((err: HttpError, _req: Request, res: Response, _next: NextFunction) => {
  if (err instanceof Error) {
    logger.error(err.message);
    const statusCode = err.status || err.status || 500;

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
