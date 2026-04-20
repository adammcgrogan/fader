export default {
  async fetch(request, env) {
    const url = new URL(request.url);
    const host = url.hostname;

    const parts = host.split(".");
    const subdomain = parts.length === 3 ? parts[0] : "";

    const origin = env.RAILWAY_ORIGIN; // e.g. o0dsrd0d.up.railway.app

    // Keep Host as fader.bio so Railway routes correctly, but resolve via Railway's origin IP
    const targetUrl = new URL(request.url);
    targetUrl.hostname = "fader.bio";

    const headers = new Headers(request.headers);
    headers.set("X-Fader-Subdomain", subdomain);
    headers.set("X-Real-IP", request.headers.get("CF-Connecting-IP") || "");

    const newRequest = new Request(targetUrl.toString(), {
      method: request.method,
      headers,
      body: request.body,
      redirect: "follow",
      cf: { resolveOverride: origin },
    });

    return fetch(newRequest);
  },
};
