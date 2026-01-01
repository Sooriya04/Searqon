const app = require("./app");
const logger = require("./utils/logger");
const port = 3001

app.listen(port, () => {
  //logger.info(`DuckDuckGo Search Service running on port ${port}`);
  console.log("server is running on http://localhost:3001/")
});
