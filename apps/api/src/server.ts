import express from "express";
import env from "../config/dotenv";

const app = express();

app.get("/", (req, res) => {
  res.json({ messasge: "Hello" });
});

app.listen(env.PORT, () => {
  console.log("Server is connecting on port:5000");
});
