const { Ollama } = require('ollama');
const readline = require('readline-sync');

const ollama = new Ollama({ host: 'http://127.0.0.1:11434' });

async function searchSearqon(rawQuery) {
  try {
    // Aggressively clean conversational words since the LLM often fails to use strict keywords
    const query = rawQuery.toLowerCase()
      .replace(/\b(what|who|where|why|how|is|are|do|does|did|can|could|will|would|tell|me|about|explain|the|a|an|and|or|but|it|people|use)\b/g, '')
      .replace(/[?.,!]/g, '')
      .replace(/\s+/g, ' ')
      .trim() || rawQuery; // fallback to raw if we stripped everything

    console.log(`\n[Tool Call] Executing Searqon search for: "${query}" (Original: "${rawQuery}")...`);
    const res = await fetch('http://localhost:3001/api/v1/unified', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ query, limit: 3 })
    });
    const data = await res.json();

    // Check if the response is an array (which /api/v2/research returns directly)
    const results = Array.isArray(data) ? data : (data.results || []);

    if (results.length === 0) {
      return JSON.stringify([{
        title: "No Results Found",
        snippet: "The search providers (DuckDuckGo/Talven) returned zero results or are currently unreachable. Try a different query or make sure your local providers are running."
      }]);
    }

    return JSON.stringify(results.map(r => ({
      title: r.title,
      url: r.url,
      source: r.source,
      snippet: r.content || (r.markdown ? r.markdown.substring(0, 300) : "No content available")
    })));
  } catch (err) {
    return "Error executing search: " + err.message;
  }
}

async function runChatbot() {
  console.log("Chatbot initialized using gemma4:e2b. Type 'exit' to quit.");

  let messages = [
    {
      role: 'system',
      content: 'You are a helpful and knowledgeable assistant. You MUST call the searqon_search tool for every user question. Once you receive the search results, use them as your primary context, but feel free to combine them with your own internal knowledge to provide a clear, direct, and comprehensive answer.'
    }
  ];

  while (true) {
    const userInput = readline.question('\nYou: ');
    if (userInput.toLowerCase() === 'exit') break;

    messages.push({
      role: 'user',
      content: userInput + "\n\n(Remember: You must call the searqon_search tool to look up information before answering.)"
    });

    try {
      let response = await ollama.chat({
        model: 'gemma4:e2b',
        messages: messages,
        tools: [{
          type: 'function',
          function: {
            name: 'searqon_search',
            description: 'Search the web using Searqon. CRITICAL: You must ONLY provide the core keywords for the search query. DO NOT pass full sentences or questions. For example, if asked "what is graphify and why is it used?", the query should simply be "graphify".',
            parameters: {
              type: 'object',
              properties: {
                query: {
                  type: 'string',
                  description: 'The short keywords to search for'
                }
              },
              required: ['query']
            }
          }
        }]
      });

      if (response.message.tool_calls && response.message.tool_calls.length > 0) {
        messages.push(response.message);
        for (const toolCall of response.message.tool_calls) {
          if (toolCall.function.name === 'searqon_search') {
            const query = toolCall.function.arguments.query;
            const searchResults = await searchSearqon(query);
            // console.log(`\n[Debug] Tool Output Sent to LLM:`, searchResults);
            // Many local models (like Gemma) ignore 'tool' role messages or fail to parse them correctly.
            // Feeding it back as a 'user' message forces the model to read the results.
            messages.push({
              role: 'user',
              content: `SEARCH SYSTEM: The tool 'searqon_search' returned the following data for your query "${query}":\n\n${searchResults}\n\nCRITICAL: Based strictly on the above data, provide a helpful answer.`
            });
          }
        }

        // Call ollama again with the tool response
        response = await ollama.chat({
          model: 'gemma4:e2b',
          messages: messages
        });
      }

      console.log(`\nBot: ${response.message.content}`);
      messages.push(response.message);
    } catch (err) {
      console.error("\nError:", err.message);
    }
  }
}

runChatbot();
