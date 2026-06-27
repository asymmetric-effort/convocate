const PORT = Number(process.env.PORT) || 8080;

const server = Bun.serve({
  port: PORT,
  async fetch(req: Request): Promise<Response> {
    const url = new URL(req.url);

    // Health check
    if (url.pathname === "/healthz") {
      return new Response(JSON.stringify({ status: "ok" }), {
        headers: { "Content-Type": "application/json" },
      });
    }

    // Serve static files from public/
    const filePath = url.pathname === "/" ? "/index.html" : url.pathname;
    const file = Bun.file(`public${filePath}`);
    if (await file.exists()) {
      return new Response(file);
    }

    // SPA fallback — return index.html for all unmatched routes
    const index = Bun.file("public/index.html");
    if (await index.exists()) {
      return new Response(index);
    }

    return new Response("Not Found", { status: 404 });
  },
});

console.log(`Convocate UI server listening on :${server.port}`);
