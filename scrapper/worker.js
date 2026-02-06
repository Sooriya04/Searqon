const scrape = require('./core');

if (require.main === module) {
  process.on('message', async (msg) => {
    const { id, url, options } = msg;

    try {
      if (!url) {
        throw new Error('URL is required');
      }

      const result = await scrape(url, options);

      process.send({
        id,
        status: 'success',
        data: result,
      });
    } catch (error) {
      process.send({
        id,
        status: 'error',
        error: error.message,
      });
    }
  });
}
