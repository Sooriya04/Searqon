require("dotenv").config();
const app = require("./app");
const { connectDB } = require("./config/database");
const port = 3001;

// Initialize PostgreSQL Connection Pool
connectDB();

app.listen(port, () => {
  console.log(`[Main API] Server is running on http://localhost:${port}/`);
});
