const { unifiedSearchPost } = require('./controller/unifiedController');

// Mock request and response
const req = {
    body: {
        query: 'test query',
        limit: 2,
        deepSearch: false
    }
};

const res = {
    status: (code) => ({
        json: (data) => console.log(`[Status ${code}]`, JSON.stringify(data, null, 2))
    }),
    json: (data) => console.log('[Success]', JSON.stringify(data, null, 2))
};

console.log('Running unified search test...');
unifiedSearchPost(req, res).catch(err => console.error('Unhandled error:', err));
