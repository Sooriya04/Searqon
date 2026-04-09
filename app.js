const express = require('express');
const v1Router = require('./routes/v1');
const crawlRoutes = require("./routes/crawl");
const searchRoutes = require("./routes/search");
const streamRoutes = require("./routes/stream");
const chatRoutes = require("./routes/chat");
const app = express();

app.use(express.json());

app.get("/", (req, res) => {
    res.send("SEARQON is running - Version: v1")
})

// Production-grade versioned API routing
app.use('/api/v1', v1Router);

app.use("/api/search", searchRoutes);
app.use("/api/search/stream", streamRoutes);
app.use("/api/chat", chatRoutes);

module.exports = app;
