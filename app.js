const express = require("express");
const searchRoutes = require("./routes/search");
const reddit = require("./routes/reddit");
const wiki = require("./routes/wiki");
const github = require("./routes/github");
const bing = require("./routes/bing");
const app = express();

app.use(express.json());

app.use("/api", searchRoutes);
app.use("/api", reddit);
app.use("/api", wiki);
app.use("/api", github )
app.use("/api", bing)


module.exports = app;