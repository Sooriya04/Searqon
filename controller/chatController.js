const { chatWithContext, chatWithContextStream } = require('../services/extractionService');

exports.handleFollowUp = async (req, res) => {
    try {
        const { messages, searchContext } = req.body;

        if (!messages || !Array.isArray(messages) || messages.length === 0) {
            return res.status(400).json({ success: false, error: 'messages array is required' });
        }

        console.log(`[Chat] Follow-up search context: processing ${messages.length} messages.`);
        
        // Pass context and history to the LLM
        const answer = await chatWithContext(messages, searchContext);

        res.json({
            success: true,
            answer
        });

    } catch (error) {
        console.error(`[Chat] Error handling follow-up: ${error.message}`);
        res.status(500).json({ success: false, error: error.message });
    }
};

exports.handleFollowUpStream = async (req, res) => {
    try {
        // SSE requests are typically GET, but for chat context POST is needed. 
        // We can handle POST request keeping the connection open for SSE.
        const { messages, searchContext } = req.body;

        if (!messages || !Array.isArray(messages) || messages.length === 0) {
            return res.status(400).json({ success: false, error: 'messages array is required' });
        }

        res.setHeader('Content-Type', 'text/event-stream');
        res.setHeader('Cache-Control', 'no-cache');
        res.setHeader('Connection', 'keep-alive');

        const stream = chatWithContextStream(messages, searchContext);
        for await (const chunkStr of stream) {
            res.write(`data: ${chunkStr}`);
        }
        
        res.write(`data: ${JSON.stringify({ type: 'done' })}\n\n`);
        res.end();

    } catch (error) {
        console.error(`[Chat] Error handling follow-up stream: ${error.message}`);
        res.write(`data: ${JSON.stringify({ type: 'error', data: error.message })}\n\n`);
        res.end();
    }
};
