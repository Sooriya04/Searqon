require("dotenv").config();
const app = require("./app");
const connectDB = require("./config/database");
const port = 3001;

connectDB();

app.listen(port, () => {
  console.log(`Server is running on http://localhost:${port}/`);
});
