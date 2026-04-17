export default {
  async fetch(request, env) {
    const url = new URL(request.url);
    const host = url.hostname; // e.g. spinmaster.fader.bio

    const parts = host.split(".");
    // *.fader.bio → 3 parts; fader.bio → 2 parts
    const subdomain = parts.length === 3 ? parts[0] : "";

    const origin = env.RAILWAY_ORIGIN; // set in Cloudflare Worker env vars
    const targetUrl = new URL(request.url);
    targetUrl.hostname = origin;

    const headers = new Headers(request.headers);
    headers.set("X-Fader-Subdomain", subdomain);
    // Pass real IP to Go app for analytics hashing
    headers.set("X-Real-IP", request.headers.get("CF-Connecting-IP") || "");

    const newRequest = new Request(targetUrl.toString(), {
      method: request.method,
      headers,
      body: request.body,
      redirect: "follow",
    });

    return fetch(newRequest);
  },
};
