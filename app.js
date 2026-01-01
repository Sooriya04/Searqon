const express = require("express");
const searchRoutes = require("./routes/search");

const app = express();

app.use(express.json());

app.use("/api", searchRoutes);

module.exports = app;