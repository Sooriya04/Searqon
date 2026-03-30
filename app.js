const express = require('express');
const searchRoutes = require('./routes/search');
const reddit = require('./routes/reddit');
const wiki = require('./routes/wiki');
const github = require('./routes/github');
const hackernew = require('./routes/hackernew');
const arixv = require('./routes/arxiv');
const pubmed = require('./routes/pubmed');
const openAlex = require("./routes/openalex")
const doaj = require("./routes/doaj");
const medrxiv = require("./routes/medrxiv");
const unified = require("./routes/unified");
const geeksforgeeks = require("./routes/geeksforgeeks");
const youtube = require("./routes/youtube");
const app = express();

app.use(express.json());

app.get("/", (req, res) => {
    res.send("SEARQON is running")
})

app.use('/api', searchRoutes);
app.use('/api', reddit);
app.use('/api', wiki);
app.use('/api', github);
app.use('/api', hackernew);
app.use('/api', arixv);
app.use('/api', pubmed);
app.use("/api", openAlex);
app.use("/api", doaj);
app.use("/api", medrxiv);
app.use("/api", unified);
app.use("/api", geeksforgeeks);
app.use("/api", youtube);

module.exports = app;
