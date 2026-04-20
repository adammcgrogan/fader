export default {
  async fetch(request, env) {
    const url = new URL(request.url);
    const host = url.hostname;

    const parts = host.split(".");
    const subdomain = parts.length === 3 ? parts[0] : "";

    // Rewrite hostname to apex so Railway routes correctly (it only knows fader.bio)
    const targetUrl = new URL(request.url);
    targetUrl.hostname = "fader.bio";

    const headers = new Headers(request.headers);
    headers.set("X-Fader-Subdomain", subdomain);
    headers.set("X-Real-IP", request.headers.get("CF-Connecting-IP") || "");

    const newRequest = new Request(targetUrl.toString(), {
      method: request.method,
      headers,
      body: request.body,
      redirect: "manual",
    });

    return fetch(newRequest);
  },
};
