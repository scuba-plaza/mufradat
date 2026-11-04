/** Calls the app's own API. Throws with the server's message when it refuses. */
export async function api<T>(url: string, init?: RequestInit): Promise<T> {
  const res = await fetch(url, init);
  const body = await res.json().catch(() => ({}));
  if (!res.ok) {
    const message = body.error ?? (Array.isArray(body.errors) ? body.errors.join(" ") : `Request failed (${res.status}).`);
    throw Object.assign(new Error(message), { errors: body.errors as string[] | undefined });
  }
  return body as T;
}

export const post = <T,>(url: string, body: unknown) =>
  api<T>(url, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
