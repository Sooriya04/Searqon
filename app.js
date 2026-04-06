const express = require('express');
const v1Router = require('./routes/v1');
const app = express();

app.use(express.json());

app.get("/", (req, res) => {
    res.send("SEARQON is running - Version: v1")
})

// Production-grade versioned API routing
app.use('/api/v1', v1Router);

module.exports = app;
