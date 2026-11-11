package scraper

import (
	"strings"
	"testing"
)

func TestStreamMarkdownBasic(t *testing.T) {
	htmlInput := `
		<!DOCTYPE html>
		<html>
		<head>
			<title>Ignore Me</title>
			<style>body { color: red; }</style>
			<script>console.log("secret");</script>
		</head>
		<body>
			<h1>Searqon Web Engine</h1>
			<p>Searqon is a <strong>fast</strong> and <em>stealthy</em> web intelligence platform.</p>
			<h2>Features</h2>
			<ul>
				<li>Zero external dependencies</li>
				<li>Dual native protocols: <a href="https://example.com/utcp">UTCP</a> and MCP</li>
			</ul>
			<ol>
				<li>Discovery</li>
				<li>Scraping</li>
				<li>Synthesis</li>
			</ol>
			<pre><code>func main() {
    fmt.Println("Hello Searqon")
}</code></pre>
			<blockquote>High performance with zero bloat.</blockquote>
		</body>
		</html>
	`

	md, truncated, err := ConvertHTMLToMarkdownStream(strings.NewReader(htmlInput), 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if truncated {
		t.Fatalf("expected truncated=false, got true")
	}

	expectedParts := []string{
		"# Searqon Web Engine",
		"Searqon is a **fast** and *stealthy* web intelligence platform.",
		"## Features",
		"* Zero external dependencies",
		"* Dual native protocols: [UTCP](https://example.com/utcp) and MCP",
		"1. Discovery",
		"2. Scraping",
		"3. Synthesis",
		"```\nfunc main() {\n    fmt.Println(\"Hello Searqon\")\n}\n```",
		"> High performance with zero bloat.",
	}

	for _, part := range expectedParts {
		if !strings.Contains(md, part) {
			t.Errorf("markdown missing expected substring: %q\nFull output:\n%s", part, md)
		}
	}

	// Ensure script and style were stripped
	if strings.Contains(md, "secret") || strings.Contains(md, "color: red") {
		t.Errorf("script or style content was not skipped: %s", md)
	}
}

func TestStreamMarkdownTables(t *testing.T) {
	htmlInput := `
		<table>
			<thead>
				<tr>
					<th>Engine</th>
					<th>Speed</th>
					<th>Memory</th>
				</tr>
			</thead>
			<tbody>
				<tr>
					<td>Searqon</td>
					<td>Sub-10ms</td>
					<td>8 MB</td>
				</tr>
				<tr>
					<td>Chromium</td>
					<td>150ms</td>
					<td>250 MB</td>
				</tr>
			</tbody>
		</table>
	`

	md, truncated, err := ConvertHTMLToMarkdownStream(strings.NewReader(htmlInput), 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if truncated {
		t.Fatalf("expected truncated=false")
	}

	if !strings.Contains(md, "| Engine | Speed | Memory |") {
		t.Errorf("table header missing: %s", md)
	}
	if !strings.Contains(md, "| --- | --- | --- |") {
		t.Errorf("table separator missing: %s", md)
	}
	if !strings.Contains(md, "| Searqon | Sub-10ms | 8 MB |") {
		t.Errorf("table row 1 missing: %s", md)
	}
	if !strings.Contains(md, "| Chromium | 150ms | 250 MB |") {
		t.Errorf("table row 2 missing: %s", md)
	}
}

func TestStreamMarkdownBoundedTruncation(t *testing.T) {
	longHTML := `<h1>Title</h1><p>` + strings.Repeat("Lorem ipsum dolor sit amet, consectetur adipiscing elit. ", 100) + `</p>`

	maxBytes := 120
	md, truncated, err := ConvertHTMLToMarkdownStream(strings.NewReader(longHTML), maxBytes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !truncated {
		t.Fatalf("expected truncated=true, got false")
	}
	if !strings.Contains(md, "[truncated]") {
		t.Errorf("expected [truncated] marker in output: %s", md)
	}
	// Content length before marker should not exceed maxBytes by more than marker overhead
	contentWithoutMarker := strings.TrimSuffix(md, defaultTruncationMarker)
	if len(contentWithoutMarker) > maxBytes+10 {
		t.Errorf("output content exceeds maxBytes (%d): %d", maxBytes, len(contentWithoutMarker))
	}
}

func TestStreamMarkdownManualCheck(t *testing.T) {
	sampleHTML := `
	<div class="main-content">
		<h1>Searqon Architecture Overview</h1>
		<p>Searqon is an <strong>agentic</strong> web retrieval layer. It provides <em>real-time</em> search and anti-bot scraping.</p>
		
		<h3>Protocol Matrix</h3>
		<table>
			<tr>
				<th>Protocol</th>
				<th>Transport</th>
				<th>Latency</th>
			</tr>
			<tr>
				<td>UTCP</td>
				<td>HTTP/SSE</td>
				<td>Sub-10ms</td>
			</tr>
			<tr>
				<td>MCP</td>
				<td>Stdio/SSE</td>
				<td>15ms</td>
			</tr>
		</table>

		<h3>Nested Pipeline Steps</h3>
		<ul>
			<li>Input Phase
				<ul>
					<li>URL normalization</li>
					<li>SSRF filter</li>
				</ul>
			</li>
			<li>Retrieval Phase
				<ol>
					<li>Fast HTTP</li>
					<li>Headless fallback</li>
				</ol>
			</li>
		</ul>

		<h3>Code Example</h3>
		<pre><code class="language-go">func StreamPipeline(r io.Reader) string {
    return "done"
}</code></pre>

		<p>For more details, visit <a href="https://searqon.dev/docs">Searqon Documentation</a>.</p>
	</div>
	`

	output, truncated, err := ConvertHTMLToMarkdownStream(strings.NewReader(sampleHTML), 0)
	if err != nil {
		t.Fatalf("conversion error: %v", err)
	}

	t.Logf("\n--- MANUAL CHECK OUTPUT (FULL) ---\n%s\n--- END FULL --- (truncated: %v)\n", output, truncated)

	// Now check bounded output (e.g. 150 bytes limit)
	boundedOut, isTrunc, err := ConvertHTMLToMarkdownStream(strings.NewReader(sampleHTML), 150)
	if err != nil {
		t.Fatalf("bounded error: %v", err)
	}
	t.Logf("\n--- MANUAL CHECK OUTPUT (BOUNDED 150 BYTES) ---\n%s\n--- END BOUNDED --- (truncated: %v)\n", boundedOut, isTrunc)
}

func BenchmarkStreamMarkdown(b *testing.B) {
	htmlInput := `
		<div class="article">
			<h1>Streaming Markdown Benchmark</h1>
			<p>Testing single-pass state machine conversion without DOM tree building.</p>
			<ul>
				<li>Item 1 with <a href="https://example.com/1">link</a></li>
				<li>Item 2 with <strong>bold</strong> text</li>
			</ul>
			<table>
				<tr><th>Col A</th><th>Col B</th></tr>
				<tr><td>Val 1</td><td>Val 2</td></tr>
			</table>
		</div>
	`

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _, _ = ConvertHTMLToMarkdownStream(strings.NewReader(htmlInput), 0)
	}
}
